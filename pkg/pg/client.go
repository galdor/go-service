package pg

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"go.n16f.net/service/pkg/influx"
)

const (
	DefaultPoolSize                     = 5
	DefaultConnectionAcquisitionTimeout = 5000 // milliseconds
)

var (
	ErrNoConnectionAvailable = errors.New("no connection available")
)

type ClientCfg struct {
	Log          *log.Logger    `json:"-"`
	InfluxClient *influx.Client `json:"-"`
	Name         string         `json:"-"`

	URI             string `json:"uri"`
	ApplicationName string `json:"application_name,omitempty"`

	PoolSize int `json:"pool_size,omitempty"`

	ConnectionAcquisitionTimeout int `json:"connection_acquisition_timeout,omitempty"` // milliseconds

	SchemaDirectory string   `json:"schema_directory"`
	SchemaNames     []string `json:"schema_names"`
}

type Client struct {
	Cfg ClientCfg
	Log *log.Logger

	Pool *pgxpool.Pool

	connectionAcquisitionTimeout time.Duration

	listenerMutex sync.Mutex
	listeners     map[string]*Listener

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func (cfg *ClientCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringURI("uri", cfg.URI)

	if cfg.PoolSize != 0 {
		// We need at least 2 connections for schema management
		v.CheckIntMinMax("pool_size", cfg.PoolSize, 2, 1000)
	}

	if cfg.ConnectionAcquisitionTimeout != 0 {
		v.CheckIntMin("connection_acquisition_timeout",
			cfg.ConnectionAcquisitionTimeout, 1)
	}

	v.WithChild("schema_names", func() {
		for i, name := range cfg.SchemaNames {
			v.CheckStringNotEmpty(i, name)
		}
	})
}

func NewClient(cfg ClientCfg) (*Client, error) {
	if cfg.Log == nil {
		cfg.Log = log.DefaultLogger("pg")
	}

	if cfg.PoolSize == 0 {
		cfg.PoolSize = DefaultPoolSize
	}

	if cfg.ConnectionAcquisitionTimeout == 0 {
		cfg.ConnectionAcquisitionTimeout = DefaultConnectionAcquisitionTimeout
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if cfg.ApplicationName != "" {
		runtimeParams := poolCfg.ConnConfig.RuntimeParams
		runtimeParams["application_name"] = cfg.ApplicationName
	}

	poolCfg.MaxConns = int32(cfg.PoolSize)

	poolCfg.MaxConnIdleTime = 10 * time.Minute
	poolCfg.MaxConnLifetimeJitter = time.Second

	cfg.Log.Info("connecting to database %q at %s:%d as %q",
		poolCfg.ConnConfig.Database,
		poolCfg.ConnConfig.Host,
		poolCfg.ConnConfig.Port,
		poolCfg.ConnConfig.User)

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	c := Client{
		Cfg: cfg,
		Log: cfg.Log,

		Pool: pool,

		listeners: make(map[string]*Listener),

		stopChan: make(chan struct{}),
	}

	c.connectionAcquisitionTimeout =
		time.Duration(cfg.ConnectionAcquisitionTimeout) * time.Millisecond

	if c.Cfg.SchemaDirectory != "" {
		if err := c.updateSchemas(); err != nil {
			c.Close()
			return nil, err
		}
	}

	if cfg.InfluxClient != nil {
		c.wg.Add(1)
		go c.influxProbeMain()
	}

	return &c, nil
}

func (c *Client) updateSchemas() error {
	for _, name := range c.Cfg.SchemaNames {
		dirPath := path.Join(c.Cfg.SchemaDirectory, name)

		if err := c.UpdateSchema(name, dirPath); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) Close() {
	close(c.stopChan)
	c.wg.Wait()

	c.listenerMutex.Lock()
	defer c.listenerMutex.Unlock()
	for _, listener := range c.listeners {
		listener.Close()
	}

	c.Pool.Close()
}

func (c *Client) WithConn(fn func(Conn) error) error {
	return c.withConn(fn)
}

func (c *Client) WithTx(fn func(Conn) error) error {
	return c.withConn(func(conn Conn) error {
		ctx := context.Background()

		if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
			return fmt.Errorf("cannot begin transaction: %w", err)
		}

		if err := fn(conn); err != nil {
			if _, rbErr := conn.Exec(ctx, "ROLLBACK"); rbErr != nil {
				// There is nothing we can do here, and we do want to return the
				// function error, so we simply log the rollback error.
				//
				// Note that when the connection is released by withConn after a
				// rollback failure, the connection will not be in an idle
				// state, and Pgx will close it, as it should be.
				c.Log.Error("cannot rollback transaction: %v", rbErr)
			}

			return err
		}

		if _, err := conn.Exec(ctx, "COMMIT"); err != nil {
			return fmt.Errorf("cannot commit transaction: %w", err)
		}

		return nil
	})
}

func (c *Client) withConn(fn func(Conn) error) error {
	ctx, cancel := context.WithTimeout(context.Background(),
		c.connectionAcquisitionTimeout)
	defer cancel()

	conn, err := c.Pool.Acquire(ctx)
	if err != nil {
		// We would like to detect connection errors to return them clearly
		// identified, but pgx is yet another one of those libraries hiding
		// their error types for inane reasons, making life harder for
		// everyone. Not much we can do here, so we will get incredibly
		// verbose error messages (including connection parameters) for each
		// connection failure.

		if errors.Is(err, context.DeadlineExceeded) {
			err = ErrNoConnectionAvailable
		}

		return fmt.Errorf("cannot acquire connection: %w", err)
	}
	defer conn.Release()

	return fn(conn)
}

func TakeAdvisoryTxLock(conn Conn, id1, id2 uint32) error {
	return Exec(conn, `SELECT pg_advisory_xact_lock($1, $2)`, id1, id2)
}

func Notify(conn Conn, channel, payload string) error {
	return Exec(conn, `SELECT pg_notify($1, $2)`, channel, payload)
}

func (c *Client) influxProbeMain() {
	defer c.wg.Done()

	timer := time.NewTicker(time.Second)
	defer timer.Stop()

	for {
		select {
		case <-c.stopChan:
			return

		case <-timer.C:
			c.sendInfluxPoints()
		}
	}
}

func (c *Client) sendInfluxPoints() {
	now := time.Now()
	stats := c.Pool.Stat()

	tags := influx.Tags{
		"client": c.Cfg.Name,
	}

	fields := influx.Fields{
		"max_nb_connections":      stats.MaxConns(),
		"nb_connections":          stats.TotalConns(),
		"nb_idle_connections":     stats.IdleConns(),
		"nb_acquired_connections": stats.AcquiredConns(),
		"nb_opening_connections":  stats.ConstructingConns(),
	}

	point := influx.NewPointWithTimestamp("pg_clients", tags, fields, now)

	c.Cfg.InfluxClient.EnqueuePoint(point)

}
