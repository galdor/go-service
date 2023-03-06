package pg

import (
	"context"
	"fmt"
	"path"

	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/sjson"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClientCfg struct {
	Log *log.Logger `json:"-"`

	URI             string `json:"uri"`
	ApplicationName string `json:"applicationName,omitempty"`

	SchemaDirectory string   `json:"schemaDirectory"`
	SchemaNames     []string `jsoN:"schemaNames"`
}

type Client struct {
	Cfg ClientCfg
	Log *log.Logger

	Pool *pgxpool.Pool
}

func (cfg *ClientCfg) ValidateJSON(v *sjson.Validator) {
	v.CheckStringURI("uri", cfg.URI)

	v.Push("schemaNames")
	for i, name := range cfg.SchemaNames {
		v.CheckStringNotEmpty(i, name)
	}
	v.Pop()
}

func NewClient(cfg ClientCfg) (*Client, error) {
	if cfg.Log == nil {
		cfg.Log = log.DefaultLogger("pg")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("invalid uri: %w", err)
	}

	if cfg.ApplicationName != "" {
		runtimeParams := poolCfg.ConnConfig.RuntimeParams
		runtimeParams["application_name"] = cfg.ApplicationName
	}

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
	}

	if c.Cfg.SchemaDirectory != "" {
		if err := c.updateSchemas(); err != nil {
			c.Close()
			return nil, err
		}
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
	c.Pool.Close()
}

func (c *Client) WithConn(fn func(Conn) error) error {
	ctx := context.Background()

	conn, err := c.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("cannot acquire connection: %w", err)
	}
	defer conn.Release()

	return fn(conn)
}

func (c *Client) WithTx(fn func(Conn) error) (err error) {
	ctx := context.Background()

	conn, acquireErr := c.Pool.Acquire(ctx)
	if acquireErr != nil {
		err = fmt.Errorf("cannot acquire connection: %w", acquireErr)
		return
	}
	defer conn.Release()

	if _, beginErr := conn.Exec(ctx, "BEGIN"); beginErr != nil {
		err = fmt.Errorf("cannot begin transaction: %w", beginErr)
		return
	}

	defer func() {
		if err != nil {
			// If an error was already signaled, do not commit
			return
		}

		if _, commitErr := conn.Exec(ctx, "COMMIT"); commitErr != nil {
			err = fmt.Errorf("cannot commit transaction: %w", commitErr)
		}
	}()

	if fnErr := fn(conn); fnErr != nil {
		err = fnErr

		if _, rollbackErr := conn.Exec(ctx, "ROLLBACK"); rollbackErr != nil {
			// There is nothing we can do here, and we do want to return the
			// function error, so we simply log the rollback error.
			c.Log.Error("cannot rollback transaction: %v", err)
		}
	}

	return
}

func TakeAdvisoryTxLock(conn Conn, id1, id2 uint32) error {
	return Exec(conn, `SELECT pg_advisory_xact_lock($1, $2)`, id1, id2)
}
