package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Conn interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type Object interface {
	FromRow(pgx.Row) error
}

type Objects interface {
	AddFromRow(pgx.Row) error
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

func QueryObject(conn Conn, obj Object, query string, args ...interface{}) error {
	ctx := context.Background()
	row := conn.QueryRow(ctx, query, args...)
	return obj.FromRow(row)
}

func QueryObjects(conn Conn, objs Objects, query string, args ...interface{}) error {
	ctx := context.Background()

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("cannot execute query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := objs.AddFromRow(rows); err != nil {
			return fmt.Errorf("cannot read row: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("cannot read query response: %w", err)
	}

	return nil
}
