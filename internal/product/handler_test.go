package product

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
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
	// Sin protección: la autenticación se prueba en el paquete auth y en TestWriteRoutesAreProtected.
	return newTestServerWith(t, repo, func(h http.Handler) http.Handler { return h })
}

func newTestServerWith(t *testing.T, repo Repository, protect func(http.Handler) http.Handler) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	NewHandler(NewService(repo), logger).RegisterRoutes(mux, protect)
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
		if page := decode[Page](t, resp); len(page.Items) != 1 || page.Total != 1 {
			t.Fatalf("unexpected page: %+v", page)
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

func TestListPagination(t *testing.T) {
	forEachRepo(t, func(t *testing.T, srv *httptest.Server) {
		names := []string{"Teclado 1", "Mouse", "teclado 2", "Monitor", "TECLADO 3", "Teclado_4", "Cable 100%"}
		for _, n := range names {
			expectStatus(t, do(t, http.MethodPost, srv.URL+"/products", `{"name":"`+n+`","price":1}`), http.StatusCreated)
		}

		list := func(t *testing.T, query string) Page {
			t.Helper()
			resp := do(t, http.MethodGet, srv.URL+"/products"+query, "")
			expectStatus(t, resp, http.StatusOK)
			return decode[Page](t, resp)
		}
		itemNames := func(p Page) []string {
			out := make([]string, len(p.Items))
			for i, it := range p.Items {
				out[i] = it.Name
			}
			return out
		}

		cases := []struct {
			query                string
			wantNames            []string
			wantPage, wantLimit  int
			wantTotal, wantPages int
		}{
			{"", names, 1, DefaultLimit, 7, 1},
			{"?limit=3", names[0:3], 1, 3, 7, 3},
			{"?limit=3&page=2", names[3:6], 2, 3, 7, 3},
			{"?limit=3&page=3", names[6:7], 3, 3, 7, 3},
			{"?limit=3&page=4", []string{}, 4, 3, 7, 3},
			// El filtro no distingue mayúsculas.
			{"?name=teclado", []string{"Teclado 1", "teclado 2", "TECLADO 3", "Teclado_4"}, 1, DefaultLimit, 4, 1},
			{"?name=TECLADO&limit=2&page=2", []string{"TECLADO 3", "Teclado_4"}, 2, 2, 4, 2},
			// Los comodines de LIKE se buscan como texto literal.
			{"?name=_", []string{"Teclado_4"}, 1, DefaultLimit, 1, 1},
			{"?name=%25", []string{"Cable 100%"}, 1, DefaultLimit, 1, 1},
			{"?name=nada", []string{}, 1, DefaultLimit, 0, 0},
		}
		for _, c := range cases {
			t.Run(c.query, func(t *testing.T) {
				p := list(t, c.query)
				if got := itemNames(p); !slices.Equal(got, c.wantNames) {
					t.Errorf("items = %q, want %q", got, c.wantNames)
				}
				if p.Page != c.wantPage || p.Limit != c.wantLimit || p.Total != c.wantTotal || p.TotalPages != c.wantPages {
					t.Errorf("page=%d limit=%d total=%d total_pages=%d, want %d %d %d %d",
						p.Page, p.Limit, p.Total, p.TotalPages, c.wantPage, c.wantLimit, c.wantTotal, c.wantPages)
				}
			})
		}

		for _, q := range []string{"?page=0", "?page=-1", "?page=abc", "?limit=0", "?limit=101", "?limit=x"} {
			t.Run("invalid "+q, func(t *testing.T) {
				expectStatus(t, do(t, http.MethodGet, srv.URL+"/products"+q, ""), http.StatusBadRequest)
			})
		}
	})
}

func TestWriteRoutesAreProtected(t *testing.T) {
	denyAll := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
	srv := newTestServerWith(t, NewMemoryRepository(), denyAll)
	id := "00000000-0000-4000-8000-000000000000"

	// Las lecturas son públicas.
	expectStatus(t, do(t, http.MethodGet, srv.URL+"/products", ""), http.StatusOK)
	expectStatus(t, do(t, http.MethodGet, srv.URL+"/products/"+id, ""), http.StatusNotFound)

	// Las escrituras pasan por protect.
	expectStatus(t, do(t, http.MethodPost, srv.URL+"/products", `{"name":"x"}`), http.StatusUnauthorized)
	expectStatus(t, do(t, http.MethodPut, srv.URL+"/products/"+id, `{"name":"x"}`), http.StatusUnauthorized)
	expectStatus(t, do(t, http.MethodDelete, srv.URL+"/products/"+id, ""), http.StatusUnauthorized)
}
