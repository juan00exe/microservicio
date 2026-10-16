package product

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository guarda los productos en PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const selectColumns = `id, name, description, price, stock, created_at, updated_at`

func (r *PostgresRepository) List(ctx context.Context) ([]Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+selectColumns+` FROM products ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, scanProduct)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []Product{}
	}
	return out, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+selectColumns+` FROM products WHERE id = $1`, id)
	if err != nil {
		return Product{}, mapErr(err)
	}
	p, err := pgx.CollectExactlyOneRow(rows, scanProduct)
	if err != nil {
		return Product{}, mapErr(err)
	}
	return p, nil
}

func (r *PostgresRepository) Create(ctx context.Context, p Product) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO products (id, name, description, price, stock, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.Name, p.Description, p.Price, p.Stock, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PostgresRepository) Update(ctx context.Context, p Product) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE products SET name = $2, description = $3, price = $4, stock = $5, updated_at = $6
		 WHERE id = $1`,
		p.ID, p.Name, p.Description, p.Price, p.Stock, p.UpdatedAt)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanProduct(row pgx.CollectableRow) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, err
}

// mapErr traduce errores de PostgreSQL a errores del dominio.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	// Un id que no es un UUID válido no puede existir en la tabla.
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "22P02" {
		return ErrNotFound
	}
	return err
}
