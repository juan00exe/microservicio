package product

import (
	"context"
	"slices"
	"strings"
	"sync"
)

// Repository abstrae el almacenamiento de productos para poder sustituirlo
// por una base de datos real sin tocar el servicio ni los handlers.
type Repository interface {
	List(ctx context.Context) ([]Product, error)
	Get(ctx context.Context, id string) (Product, error)
	Create(ctx context.Context, p Product) error
	Update(ctx context.Context, p Product) error
	Delete(ctx context.Context, id string) error
}

// MemoryRepository guarda los productos en memoria; es seguro para uso concurrente.
type MemoryRepository struct {
	mu    sync.RWMutex
	items map[string]Product
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: make(map[string]Product)}
}

func (r *MemoryRepository) List(_ context.Context) ([]Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Product, 0, len(r.items))
	for _, p := range r.items {
		out = append(out, p)
	}
	// Orden estable para respuestas predecibles.
	slices.SortFunc(out, func(a, b Product) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (r *MemoryRepository) Get(_ context.Context, id string) (Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.items[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return p, nil
}

func (r *MemoryRepository) Create(_ context.Context, p Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[p.ID] = p
	return nil
}

func (r *MemoryRepository) Update(_ context.Context, p Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[p.ID]; !ok {
		return ErrNotFound
	}
	r.items[p.ID] = p
	return nil
}

func (r *MemoryRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}
