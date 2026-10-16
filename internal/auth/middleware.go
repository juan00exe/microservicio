package auth

import (
	"context"
	"net/http"
	"strings"

	"microservicio/internal/httpx"
)

type claimsKey struct{}

// ClaimsFrom devuelve los claims del token de la petición, si los hay.
func ClaimsFrom(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}

// RequireAuth exige un token válido en "Authorization: Bearer <token>" y deja
// sus claims en el contexto de la petición.
func (m *TokenManager) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			unauthorized(w, "missing bearer token")
			return
		}
		claims, err := m.Parse(token)
		if err != nil {
			unauthorized(w, ErrInvalidToken.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey{}, claims)))
	})
}

// RequireRole exige un token válido cuyo rol sea role; si no, responde 403.
func (m *TokenManager) RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, _ := ClaimsFrom(r.Context()); c.Role != role {
				httpx.WriteError(w, http.StatusForbidden, "requires role "+string(role))
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="microservicio"`)
	httpx.WriteError(w, http.StatusUnauthorized, msg)
}
