package product

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forEachRepo ejecuta fn contra cada implementación de Repository disponible.
// PostgreSQL solo se incluye si TEST_DATABASE_URL está definida.
func forEachRepo(t *testing.T, fn func(t *testing.T, srv *httptest.Server)) {
	t.Run("memory", func(t *testing.T) {
		fn(t, newTestServer(t, NewMemoryRepository()))
	})
	t.Run("postgres", func(t *testing.T) {
		fn(t, newTestServer(t, newPostgresTestRepo(t)))
	})
}

func newTestServer(t *testing.T, repo Repository) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	NewHandler(NewService(repo), logger).RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func expectStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

func TestCRUD(t *testing.T) {
	forEachRepo(t, func(t *testing.T, srv *httptest.Server) {
		// Crear
		resp := do(t, http.MethodPost, srv.URL+"/products", `{"name":"Teclado","price":49.9,"stock":10}`)
		expectStatus(t, resp, http.StatusCreated)
		created := decode[Product](t, resp)
		if created.ID == "" || created.Name != "Teclado" {
			t.Fatalf("unexpected product: %+v", created)
		}
		if loc := resp.Header.Get("Location"); loc != "/products/"+created.ID {
			t.Fatalf("Location = %q", loc)
		}

		// Listar
		resp = do(t, http.MethodGet, srv.URL+"/products", "")
		expectStatus(t, resp, http.StatusOK)
		if list := decode[[]Product](t, resp); len(list) != 1 {
			t.Fatalf("len(list) = %d, want 1", len(list))
		}

		// Obtener
		resp = do(t, http.MethodGet, srv.URL+"/products/"+created.ID, "")
		expectStatus(t, resp, http.StatusOK)

		// Actualizar
		resp = do(t, http.MethodPut, srv.URL+"/products/"+created.ID, `{"name":"Teclado mecánico","price":79.9,"stock":5}`)
		expectStatus(t, resp, http.StatusOK)
		if updated := decode[Product](t, resp); updated.Name != "Teclado mecánico" || updated.Stock != 5 {
			t.Fatalf("unexpected update: %+v", updated)
		}

		// Borrar
		resp = do(t, http.MethodDelete, srv.URL+"/products/"+created.ID, "")
		expectStatus(t, resp, http.StatusNoContent)

		// Ya no existe
		resp = do(t, http.MethodGet, srv.URL+"/products/"+created.ID, "")
		expectStatus(t, resp, http.StatusNotFound)
	})
}

func TestNotFound(t *testing.T) {
	forEachRepo(t, func(t *testing.T, srv *httptest.Server) {
		// Un id con formato inválido y un UUID válido que no existe.
		for _, id := range []string{"nope", "00000000-0000-4000-8000-000000000000"} {
			expectStatus(t, do(t, http.MethodGet, srv.URL+"/products/"+id, ""), http.StatusNotFound)
			expectStatus(t, do(t, http.MethodPut, srv.URL+"/products/"+id, `{"name":"x"}`), http.StatusNotFound)
			expectStatus(t, do(t, http.MethodDelete, srv.URL+"/products/"+id, ""), http.StatusNotFound)
		}
	})
}

func TestValidation(t *testing.T) {
	forEachRepo(t, func(t *testing.T, srv *httptest.Server) {
		cases := map[string]string{
			"empty name":     `{"name":"  ","price":1}`,
			"negative price": `{"name":"x","price":-1}`,
			"negative stock": `{"name":"x","stock":-1}`,
			"malformed":      `{"name":`,
			"unknown field":  `{"name":"x","color":"red"}`,
		}
		for name, body := range cases {
			t.Run(name, func(t *testing.T) {
				resp := do(t, http.MethodPost, srv.URL+"/products", body)
				expectStatus(t, resp, http.StatusBadRequest)
				if e := decode[map[string]string](t, resp); e["error"] == "" {
					t.Fatal("missing error message")
				}
			})
		}
	})
}
