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

// LoginLimits configura el bloqueo por logins fallidos. Un máximo de 0 desactiva ese límite.
type LoginLimits struct {
	MaxPerEmail int           // fallos por email antes de bloquear esa cuenta
	MaxPerIP    int           // fallos por IP (con cualquier email) antes de bloquear esa IP
	Lockout     time.Duration // ventana en la que se cuentan los fallos y duración del bloqueo
}

// TooManyAttemptsError indica que el login está bloqueado temporalmente.
type TooManyAttemptsError struct {
	RetryAfter time.Duration
}

func (e *TooManyAttemptsError) Error() string {
	return "too many failed login attempts, try again later"
}

// Service contiene la lógica de registro y login.
type Service struct {
	repo       Repository
	tokens     *TokenManager
	bcryptCost int
	byEmail    *Limiter
	byIP       *Limiter
}

func NewService(repo Repository, tokens *TokenManager, limits LoginLimits) *Service {
	return &Service{
		repo:       repo,
		tokens:     tokens,
		bcryptCost: bcrypt.DefaultCost,
		byEmail:    NewLimiter(limits.MaxPerEmail, limits.Lockout),
		byIP:       NewLimiter(limits.MaxPerIP, limits.Lockout),
	}
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

// Login comprueba las credenciales y emite un JWT. Tras demasiados fallos
// devuelve *TooManyAttemptsError, incluso si la contraseña es correcta.
func (s *Service) Login(ctx context.Context, c Credentials, clientIP string) (Token, error) {
	c = c.normalized()
	if wait := max(s.byEmail.Blocked(c.Email), s.byIP.Blocked(clientIP)); wait > 0 {
		return Token{}, &TooManyAttemptsError{RetryAfter: wait}
	}

	u, err := s.authenticate(ctx, c)
	if errors.Is(err, ErrInvalidCredentials) {
		s.byEmail.Fail(c.Email)
		if clientIP != "" {
			s.byIP.Fail(clientIP)
		}
		return Token{}, err
	}
	if err != nil {
		return Token{}, err
	}
	// El contador de la IP no se reinicia: si no, un atacante con una cuenta
	// válida podría intercalar logins correctos para probar otras cuentas.
	s.byEmail.Reset(c.Email)

	tok, err := s.tokens.Issue(u)
	if err != nil {
		return Token{}, err
	}
	return Token{AccessToken: tok, TokenType: "Bearer", ExpiresIn: int(s.tokens.TTL().Seconds())}, nil
}

// authenticate devuelve el usuario si email y contraseña coinciden.
func (s *Service) authenticate(ctx context.Context, c Credentials) (User, error) {
	u, err := s.repo.GetByEmail(ctx, c.Email)
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(c.Password))
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(c.Password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	return u, nil
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
