package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "microservicio"

// MinSecretLen es la longitud mínima del secreto HS256 (256 bits).
const MinSecretLen = 32

// Claims es el contenido del JWT. El id del usuario va en "sub".
type Claims struct {
	Email string `json:"email"`
	Role  Role   `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager firma y verifica JWT con HS256.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret []byte, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < MinSecretLen {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes", MinSecretLen)
	}
	return &TokenManager{secret: secret, ttl: ttl, now: time.Now}, nil
}

// TTL es el tiempo de vida de los tokens emitidos.
func (m *TokenManager) TTL() time.Duration { return m.ttl }

// Issue emite un token para el usuario.
func (m *TokenManager) Issue(u User) (string, error) {
	now := m.now()
	claims := Claims{
		Email: u.Email,
		Role:  u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse verifica firma, algoritmo, emisor y expiración, y devuelve los claims.
func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		// Fijar el algoritmo evita ataques con "alg: none" o cambiando a otro algoritmo.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.Subject == "" || (claims.Role != RoleAdmin && claims.Role != RoleUser) {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
