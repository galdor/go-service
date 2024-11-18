package pg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.n16f.net/log"
)

func TestClientWithTx(t *testing.T) {
	require := require.New(t)

	clientCfg := ClientCfg{
		Log:             log.DefaultLogger("pg"),
		Name:            "test",
		URI:             "postgres://postgres:postgres@localhost:5432/service",
		ApplicationName: "test",
	}

	client, err := NewClient(clientCfg)
	require.NoError(err)

	err = client.WithConn(func(conn Conn) error {
		query := `DROP TABLE IF EXISTS foo`
		if err := Exec(conn, query); err != nil {
			return err
		}

		query = `CREATE TABLE foo (i INT)`
		if err := Exec(conn, query); err != nil {
			return err
		}

		return nil
	})
	require.NoError(err)

	// Transactions are not committed if the function panics
	func() {
		defer func() {
			recover()
		}()

		err = client.WithTx(func(conn Conn) error {
			query := `INSERT INTO foo (i) VALUES (1)`
			if err := Exec(conn, query); err != nil {
				return err
			}

			panic("test")
		})
		require.NoError(err)
	}()

	var count int
	err = client.WithConn(func(conn Conn) error {
		query := `SELECT COUNT(*) FROM foo`
		err := QueryRow(conn, query).Scan(&count)
		if err != nil {
			return err
		}

		return nil
	})
	require.NoError(err)
	require.Equal(0, count)
}
