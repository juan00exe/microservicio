package product

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// Service contiene la lógica de negocio de productos.
type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	// Se trunca a microsegundos, la precisión de TIMESTAMPTZ en PostgreSQL,
	// para que lo devuelto al crear coincida con lo leído después.
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }}
}

func (s *Service) List(ctx context.Context) ([]Product, error) {
	return s.repo.List(ctx)
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
