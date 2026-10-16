// Package docs sirve la especificación OpenAPI y la interfaz Swagger UI.
package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var spec []byte

//go:embed swagger.html
var swaggerHTML []byte

// RegisterRoutes expone /openapi.yaml y Swagger UI en /docs.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(swaggerHTML)
	})
	mux.Handle("GET /docs/", http.RedirectHandler("/docs", http.StatusMovedPermanently))
}
