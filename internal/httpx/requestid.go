package httpx

import (
	"context"
	"log/slog"
	"net/http"

	"microservicio/internal/ids"
)

// RequestIDHeader es la cabecera con la que se recibe y se devuelve el ID de la petición.
const RequestIDHeader = "X-Request-ID"

const maxRequestIDLen = 128

type requestIDKey struct{}

// RequestIDFrom devuelve el ID de la petición guardado en el contexto, o "" si no hay.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// RequestID asigna un ID a cada petición: reutiliza el de la cabecera X-Request-ID
// si es válido (para seguir una petición entre servicios) o genera uno nuevo.
// Lo devuelve en la respuesta y lo guarda en el contexto para los logs.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			id = ids.New()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// validRequestID acepta IDs cortos con caracteres seguros, para que un cliente
// no pueda meter texto arbitrario en los logs ni en las cabeceras.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, c := range []byte(id) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == ':':
		default:
			return false
		}
	}
	return true
}

// ContextHandler envuelve un slog.Handler y añade request_id a cada log que se
// emita con un contexto de petición (logger.InfoContext, ErrorContext…).
type ContextHandler struct {
	slog.Handler
}

func NewContextHandler(h slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: h}
}

func (h *ContextHandler) Handle(ctx context.Context, rec slog.Record) error {
	if id := RequestIDFrom(ctx); id != "" {
		rec.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, rec)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}
