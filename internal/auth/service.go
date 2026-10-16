package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"microservicio/internal/ids"
)

// Token es la respuesta de un login correcto.
type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // segundos
}

// Service contiene la lógica de registro y login.
type Service struct {
	repo       Repository
	tokens     *TokenManager
	bcryptCost int
}

func NewService(repo Repository, tokens *TokenManager) *Service {
	return &Service{repo: repo, tokens: tokens, bcryptCost: bcrypt.DefaultCost}
}

// Register crea una cuenta con rol user.
func (s *Service) Register(ctx context.Context, c Credentials) (User, error) {
	return s.create(ctx, c, RoleUser)
}

func (s *Service) create(ctx context.Context, c Credentials, role Role) (User, error) {
	c = c.normalized()
	if err := c.Validate(); err != nil {
		return User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), s.bcryptCost)
	if err != nil {
		return User{}, err
	}
	u := User{
		ID:           ids.New(),
		Email:        c.Email,
		Role:         role,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return User{}, err
	}
	return u, nil
}

// dummyHash se compara cuando el email no existe, para que el login tarde lo
// mismo exista o no la cuenta y no se pueda averiguar qué emails están registrados.
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	return h
})

// Login comprueba las credenciales y emite un JWT.
func (s *Service) Login(ctx context.Context, c Credentials) (Token, error) {
	c = c.normalized()
	u, err := s.repo.GetByEmail(ctx, c.Email)
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(c.Password))
		return Token{}, ErrInvalidCredentials
	}
	if err != nil {
		return Token{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(c.Password)) != nil {
		return Token{}, ErrInvalidCredentials
	}
	tok, err := s.tokens.Issue(u)
	if err != nil {
		return Token{}, err
	}
	return Token{AccessToken: tok, TokenType: "Bearer", ExpiresIn: int(s.tokens.TTL().Seconds())}, nil
}

// Get devuelve un usuario por id.
func (s *Service) Get(ctx context.Context, id string) (User, error) {
	return s.repo.GetByID(ctx, id)
}

// EnsureAdmin crea la cuenta admin si el email no existe todavía. Si ya existe
// no la modifica y la devuelve tal cual (created = false).
func (s *Service) EnsureAdmin(ctx context.Context, email, password string) (u User, created bool, err error) {
	c := Credentials{Email: email, Password: password}.normalized()
	u, err = s.repo.GetByEmail(ctx, c.Email)
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, false, err
	}
	u, err = s.create(ctx, c, RoleAdmin)
	if errors.Is(err, ErrEmailTaken) {
		// Otra instancia lo creó a la vez.
		u, err = s.repo.GetByEmail(ctx, c.Email)
		return u, false, err
	}
	return u, err == nil, err
}
