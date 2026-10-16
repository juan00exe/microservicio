package product

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"microservicio/internal/database"
)

// newPostgresTestRepo conecta a TEST_DATABASE_URL, aplica migraciones y vacía
// la tabla. Omite el test si la variable no está definida.
func newPostgresTestRepo(t *testing.T) *PostgresRepository {
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
	if _, err := pool.Exec(ctx, `TRUNCATE products`); err != nil {
		t.Fatal(err)
	}
	return NewPostgresRepository(pool)
}
