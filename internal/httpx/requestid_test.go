package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newLoggedHandler monta la misma cadena de middleware que el servidor y
// devuelve el buffer donde quedan los logs en JSON.
func newLoggedHandler(next http.HandlerFunc) (http.Handler, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil)))
	return RequestID(Recover(logger, Logging(logger, next))), &buf
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.Lines(buf.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestRequestIDGenerated(t *testing.T) {
	var seen string
	h, buf := newLoggedHandler(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	id := rec.Header().Get(RequestIDHeader)
	if len(id) != 36 {
		t.Fatalf("expected a generated UUID, got %q", id)
	}
	if seen != id {
		t.Fatalf("context id = %q, header id = %q", seen, id)
	}
	lines := logLines(t, buf)
	if len(lines) != 1 || lines[0]["request_id"] != id {
		t.Fatalf("request log must include request_id %q: %v", id, lines)
	}
}

func TestRequestIDFromClient(t *testing.T) {
	cases := map[string]bool{
		"abc-123":                true,
		"svc.pedidos:42_A":       true,
		strings.Repeat("a", 128): true,
		strings.Repeat("a", 129): false,
		"con espacio":            false,
		"salto\nde línea":        false,
		`{"inyectado":"json"}`:   false,
		"<script>":               false,
	}
	for incoming, keep := range cases {
		h, _ := newLoggedHandler(func(http.ResponseWriter, *http.Request) {})
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set(RequestIDHeader, incoming)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		got := rec.Header().Get(RequestIDHeader)
		if keep && got != incoming {
			t.Errorf("%q: should be reused, got %q", incoming, got)
		}
		if !keep && (got == incoming || len(got) != 36) {
			t.Errorf("%q: should be replaced by a new id, got %q", incoming, got)
		}
	}
}

func TestPanicLogIncludesRequestID(t *testing.T) {
	h, buf := newLoggedHandler(func(http.ResponseWriter, *http.Request) { panic("boom") })
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "traza-1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || rec.Header().Get(RequestIDHeader) != "traza-1" {
		t.Fatalf("status = %d, id = %q", rec.Code, rec.Header().Get(RequestIDHeader))
	}
	var panicLog map[string]any
	for _, l := range logLines(t, buf) {
		if l["msg"] == "panic" {
			panicLog = l
		}
	}
	if panicLog == nil || panicLog["request_id"] != "traza-1" {
		t.Fatalf("panic log must include request_id: %s", buf.String())
	}
}

func TestContextHandlerWithoutRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil))).With("component", "x")
	logger.Info("startup")
	lines := logLines(t, &buf)
	if _, ok := lines[0]["request_id"]; ok || lines[0]["component"] != "x" {
		t.Fatalf("unexpected log: %v", lines[0])
	}
}
