package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func newTestTokens(t *testing.T) *TokenManager {
	t.Helper()
	tm, err := NewTokenManager(testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

var testUser = User{ID: "00000000-0000-4000-8000-000000000001", Email: "a@example.com", Role: RoleAdmin}

func TestTokenRoundTrip(t *testing.T) {
	tm := newTestTokens(t)
	tok, err := tm.Issue(testUser)
	if err != nil {
		t.Fatal(err)
	}
	c, err := tm.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != testUser.ID || c.Email != testUser.Email || c.Role != RoleAdmin || c.Issuer != issuer {
		t.Fatalf("unexpected claims: %+v", c)
	}
}

func TestNewTokenManagerRejectsShortSecret(t *testing.T) {
	if _, err := NewTokenManager([]byte("short"), time.Hour); err == nil {
		t.Fatal("expected error for short secret")
	}
}

// sign firma claims arbitrarios para simular tokens manipulados o de terceros.
func sign(t *testing.T, method jwt.SigningMethod, key any, claims Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() Claims {
	now := time.Now()
	return Claims{
		Email: testUser.Email,
		Role:  RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   testUser.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
}

func TestTokenRejected(t *testing.T) {
	tm := newTestTokens(t)

	expired := newTestTokens(t)
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expiredTok, _ := expired.Issue(testUser)

	good, _ := tm.Issue(testUser)
	parts := strings.Split(good, ".")
	// Cambia un carácter del payload sin volver a firmar.
	payload := []byte(parts[1])
	if payload[10] == 'A' {
		payload[10] = 'B'
	} else {
		payload[10] = 'A'
	}
	tampered := parts[0] + "." + string(payload) + "." + parts[2]

	withClaims := func(mut func(*Claims)) string {
		c := validClaims()
		mut(&c)
		return sign(t, jwt.SigningMethodHS256, testSecret, c)
	}

	cases := map[string]string{
		"empty":           "",
		"garbage":         "not-a-jwt",
		"expired":         expiredTok,
		"tampered":        tampered,
		"wrong secret":    sign(t, jwt.SigningMethodHS256, []byte("another-secret-another-secret-123"), validClaims()),
		"alg none":        sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims()),
		"other algorithm": sign(t, jwt.SigningMethodHS512, testSecret, validClaims()),
		"wrong issuer":    withClaims(func(c *Claims) { c.Issuer = "otro" }),
		"no expiration":   withClaims(func(c *Claims) { c.ExpiresAt = nil }),
		"no subject":      withClaims(func(c *Claims) { c.Subject = "" }),
		"unknown role":    withClaims(func(c *Claims) { c.Role = "superadmin" }),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := tm.Parse(tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}
