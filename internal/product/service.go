package product

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service contiene la lógica de negocio de productos.
type Service struct {
	repo Repository

	mu   sync.Mutex
	last time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// now devuelve la hora actual en UTC truncada a microsegundos (la precisión de
// TIMESTAMPTZ en PostgreSQL) y estrictamente creciente dentro de este proceso,
// para que el listado por fecha de creación respete el orden de inserción.
func (s *Service) now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := time.Now().UTC().Truncate(time.Microsecond)
	if !t.After(s.last) {
		t = s.last.Add(time.Microsecond)
	}
	s.last = t
	return t
}

func (s *Service) List(ctx context.Context, params ListParams) (Page, error) {
	if err := params.Validate(); err != nil {
		return Page{}, err
	}
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return Page{}, err
	}
	return Page{
		Items:      items,
		Page:       params.Page,
		Limit:      params.Limit,
		Total:      total,
		TotalPages: (total + params.Limit - 1) / params.Limit,
	}, nil
}

func (s *Service) Get(ctx context.Context, id string) (Product, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, in Input) (Product, error) {
	if err := in.Validate(); err != nil {
		return Product{}, err
	}
	now := s.now()
	p := Product{
		ID:          newID(),
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		Price:       in.Price,
		Stock:       in.Stock,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return Product{}, err
	}
	return p, nil
}

func (s *Service) Update(ctx context.Context, id string, in Input) (Product, error) {
	if err := in.Validate(); err != nil {
		return Product{}, err
	}
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return Product{}, err
	}
	p.Name = strings.TrimSpace(in.Name)
	p.Description = in.Description
	p.Price = in.Price
	p.Stock = in.Stock
	p.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, p); err != nil {
		return Product{}, err
	}
	return p, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// newID genera un UUID v4 usando solo la librería estándar.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
