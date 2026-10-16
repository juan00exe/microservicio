package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository abstrae el almacenamiento de usuarios. Los emails llegan ya normalizados.
type Repository interface {
	// Create guarda un usuario nuevo; devuelve ErrEmailTaken si el email ya existe.
	Create(ctx context.Context, u User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
}

// MemoryRepository guarda los usuarios en memoria; es seguro para uso concurrente.
type MemoryRepository struct {
	mu      sync.RWMutex
	byID    map[string]User
	byEmail map[string]string // email -> id
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{byID: make(map[string]User), byEmail: make(map[string]string)}
}

func (r *MemoryRepository) Create(_ context.Context, u User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byEmail[u.Email]; ok {
		return ErrEmailTaken
	}
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u.ID
	return nil
}

func (r *MemoryRepository) GetByEmail(_ context.Context, email string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return r.byID[id], nil
}

func (r *MemoryRepository) GetByID(_ context.Context, id string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

// PostgresRepository guarda los usuarios en PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const userColumns = `id, email, role, password_hash, created_at`

func (r *PostgresRepository) Create(ctx context.Context, u User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, role, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)`,
		u.ID, u.Email, u.Role, u.PasswordHash, u.CreatedAt)
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		return ErrEmailTaken
	}
	return err
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
}

func (r *PostgresRepository) getOne(ctx context.Context, query string, arg string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx, query, arg).Scan(&u.ID, &u.Email, &u.Role, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	// Un id que no es un UUID válido no puede existir en la tabla.
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "22P02" {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}
