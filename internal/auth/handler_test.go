package auth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"microservicio/internal/database/dbtest"
	"microservicio/internal/httpx"
)

type testEnv struct {
	srv *httptest.Server
	svc *Service
}

// forEachRepo ejecuta fn contra cada implementación de Repository disponible.
// PostgreSQL solo se incluye si TEST_DATABASE_URL está definida.
func forEachRepo(t *testing.T, fn func(t *testing.T, env testEnv)) {
	t.Run("memory", func(t *testing.T) {
		fn(t, newTestEnv(t, NewMemoryRepository()))
	})
	t.Run("postgres", func(t *testing.T) {
		fn(t, newTestEnv(t, NewPostgresRepository(dbtest.Pool(t, "users"))))
	})
}

// newTestEnv monta las rutas /auth y una ruta /admin-only protegida con RequireRole.
func newTestEnv(t *testing.T, repo Repository) testEnv {
	t.Helper()
	tokens := newTestTokens(t)
	svc := NewService(repo, tokens)
	svc.bcryptCost = bcrypt.MinCost // tests rápidos

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	NewHandler(svc, tokens, logger).RegisterRoutes(mux)
	mux.Handle("GET /admin-only", tokens.RequireRole(RoleAdmin)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			c, _ := ClaimsFrom(r.Context())
			httpx.WriteJSON(w, http.StatusOK, map[string]string{"email": c.Email})
		})))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return testEnv{srv: srv, svc: svc}
}

func request(t *testing.T, method, url, token, body string) (*http.Response, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func wantStatus(t *testing.T, resp *http.Response, body map[string]any, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d (body: %v)", resp.StatusCode, want, body)
	}
}

func login(t *testing.T, env testEnv, email, password string) string {
	t.Helper()
	resp, body := request(t, http.MethodPost, env.srv.URL+"/auth/login", "",
		`{"email":"`+email+`","password":"`+password+`"}`)
	wantStatus(t, resp, body, http.StatusOK)
	tok, _ := body["access_token"].(string)
	if tok == "" || body["token_type"] != "Bearer" || body["expires_in"] != float64(3600) {
		t.Fatalf("unexpected login response: %v", body)
	}
	return tok
}

func TestRegisterLoginAndRoles(t *testing.T) {
	forEachRepo(t, func(t *testing.T, env testEnv) {
		url := env.srv.URL

		// Registro: el email se normaliza y nunca se devuelve el hash.
		resp, body := request(t, http.MethodPost, url+"/auth/register", "",
			`{"email":"  Ana@Example.com ","password":"secreto123"}`)
		wantStatus(t, resp, body, http.StatusCreated)
		if body["email"] != "ana@example.com" || body["role"] != "user" || body["id"] == "" {
			t.Fatalf("unexpected user: %v", body)
		}
		if _, ok := body["password_hash"]; ok {
			t.Fatal("password hash leaked in response")
		}

		// El mismo email (con otras mayúsculas) ya está registrado.
		resp, body = request(t, http.MethodPost, url+"/auth/register", "",
			`{"email":"ANA@example.com","password":"otraclave1"}`)
		wantStatus(t, resp, body, http.StatusConflict)

		// Login de user: puede ver su perfil pero no acceder a rutas de admin.
		userTok := login(t, env, "ANA@example.com", "secreto123")
		resp, body = request(t, http.MethodGet, url+"/auth/me", userTok, "")
		wantStatus(t, resp, body, http.StatusOK)
		if body["email"] != "ana@example.com" || body["role"] != "user" {
			t.Fatalf("unexpected /auth/me: %v", body)
		}
		resp, body = request(t, http.MethodGet, url+"/admin-only", userTok, "")
		wantStatus(t, resp, body, http.StatusForbidden)

		// Admin creado al arrancar: accede a las rutas de admin.
		ctx := context.Background()
		admin, created, err := env.svc.EnsureAdmin(ctx, "Admin@Example.com", "admin-clave")
		if err != nil || !created || admin.Role != RoleAdmin {
			t.Fatalf("EnsureAdmin = %+v, %v, %v", admin, created, err)
		}
		// Es idempotente: la segunda vez no crea nada.
		if _, created, err := env.svc.EnsureAdmin(ctx, "admin@example.com", "otra-clave"); err != nil || created {
			t.Fatalf("second EnsureAdmin: created=%v err=%v", created, err)
		}
		adminTok := login(t, env, "admin@example.com", "admin-clave")
		resp, body = request(t, http.MethodGet, url+"/admin-only", adminTok, "")
		wantStatus(t, resp, body, http.StatusOK)
		if body["email"] != "admin@example.com" {
			t.Fatalf("claims not in context: %v", body)
		}
	})
}

func TestLoginFailures(t *testing.T) {
	forEachRepo(t, func(t *testing.T, env testEnv) {
		url := env.srv.URL
		resp, body := request(t, http.MethodPost, url+"/auth/register", "", `{"email":"bob@example.com","password":"secreto123"}`)
		wantStatus(t, resp, body, http.StatusCreated)

		for name, creds := range map[string]string{
			"wrong password": `{"email":"bob@example.com","password":"incorrecta"}`,
			"unknown email":  `{"email":"nadie@example.com","password":"secreto123"}`,
		} {
			t.Run(name, func(t *testing.T) {
				resp, body := request(t, http.MethodPost, url+"/auth/login", "", creds)
				wantStatus(t, resp, body, http.StatusUnauthorized)
				// Mismo mensaje en ambos casos para no revelar qué emails existen.
				if body["error"] != ErrInvalidCredentials.Error() {
					t.Fatalf("error = %v", body["error"])
				}
			})
		}
	})
}

func TestRegisterValidation(t *testing.T) {
	env := newTestEnv(t, NewMemoryRepository())
	for name, body := range map[string]string{
		"invalid email":  `{"email":"no-es-un-email","password":"secreto123"}`,
		"display name":   `{"email":"Ana <ana@example.com>","password":"secreto123"}`,
		"short password": `{"email":"ana@example.com","password":"corta"}`,
		"long password":  `{"email":"ana@example.com","password":"` + strings.Repeat("x", 73) + `"}`,
		"unknown field":  `{"email":"ana@example.com","password":"secreto123","role":"admin"}`,
		"malformed":      `{"email":`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, out := request(t, http.MethodPost, env.srv.URL+"/auth/register", "", body)
			wantStatus(t, resp, out, http.StatusBadRequest)
		})
	}
}

func TestProtectedRouteWithoutValidToken(t *testing.T) {
	env := newTestEnv(t, NewMemoryRepository())
	for name, header := range map[string]string{
		"no header":     "",
		"wrong scheme":  "Basic dXNlcjpwYXNz",
		"empty bearer":  "Bearer ",
		"invalid token": "Bearer abc.def.ghi",
	} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, env.srv.URL+"/auth/me", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", resp.StatusCode)
			}
			if resp.Header.Get("WWW-Authenticate") == "" {
				t.Fatal("missing WWW-Authenticate header")
			}
		})
	}
}
