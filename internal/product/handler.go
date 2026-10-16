package product

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"microservicio/internal/httpx"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Handler expone el servicio de productos por HTTP.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// RegisterRoutes registra las rutas de productos en el mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /products", h.list)
	mux.HandleFunc("POST /products", h.create)
	mux.HandleFunc("GET /products/{id}", h.get)
	mux.HandleFunc("PUT /products/{id}", h.update)
	mux.HandleFunc("DELETE /products/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := ListParams{Name: strings.TrimSpace(q.Get("name"))}
	var err error
	if params.Page, err = intParam(q.Get("page"), 1); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "page must be an integer")
		return
	}
	if params.Limit, err = intParam(q.Get("limit"), DefaultLimit); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}

	page, err := h.svc.List(r.Context(), params)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

// intParam convierte un parámetro de query a entero, usando def si viene vacío.
func intParam(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	return strconv.Atoi(s)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	w.Header().Set("Location", "/products/"+p.ID)
	httpx.WriteJSON(w, http.StatusCreated, p)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Update(r.Context(), r.PathValue("id"), in)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeInput(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return Input{}, false
	}
	return in, true
}

func (h *Handler) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalid):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		h.logger.Error("unexpected error", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
