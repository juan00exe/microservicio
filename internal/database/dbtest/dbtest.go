// Package dbtest ayuda a los tests a usar una base de datos PostgreSQL real.
package dbtest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservicio/internal/database"
)

// Pool conecta a TEST_DATABASE_URL, aplica las migraciones y vacía las tablas
// indicadas. Omite el test si la variable no está definida.
func Pool(t *testing.T, tables ...string) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, err := database.Connect(ctx, url, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if err := database.Migrate(ctx, pool, logger); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if _, err := pool.Exec(ctx, `TRUNCATE `+pgx.Identifier{table}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}
