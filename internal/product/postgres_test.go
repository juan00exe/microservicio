package product

import (
	"testing"

	"microservicio/internal/database/dbtest"
)

// newPostgresTestRepo devuelve un repositorio sobre TEST_DATABASE_URL con la
// tabla products vacía. Omite el test si la variable no está definida.
func newPostgresTestRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	return NewPostgresRepository(dbtest.Pool(t, "products"))
}
