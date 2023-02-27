package pg

import (
	"context"
	"fmt"

	"github.com/galdor/go-service/pkg/log"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClientCfg struct {
	Log *log.Logger `json:"-"`

	URI             string `json:"uri"`
	ApplicationName string `json:"application_name,omitempty"`
}

type Client struct {
	Cfg ClientCfg
	Log *log.Logger

	Pool *pgxpool.Pool
}

func NewClient(cfg ClientCfg) (*Client, error) {
	if cfg.Log == nil {
		cfg.Log = log.DefaultLogger("pg")
	}

	if cfg.URI == "" {
		return nil, fmt.Errorf("missing or empty uri")
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

	return &c, nil
}

func (c *Client) Close() {
	c.Pool.Close()
}
