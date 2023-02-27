package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Conn interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func Exec(conn Conn, query string, args ...interface{}) (err error) {
	ctx := context.Background()
	_, err = conn.Exec(ctx, query, args...)
	return
}

func Exec2(conn Conn, query string, args ...interface{}) (int64, error) {
	ctx := context.Background()

	tag, err := conn.Exec(ctx, query, args...)
	if err != nil {
		return -1, err
	}

	return tag.RowsAffected(), nil
}

func Query(conn Conn, query string, args ...interface{}) (pgx.Rows, error) {
	ctx := context.Background()
	return conn.Query(ctx, query, args...)
}

func QueryRow(conn Conn, query string, args ...interface{}) pgx.Row {
	ctx := context.Background()
	return conn.QueryRow(ctx, query, args...)
}
