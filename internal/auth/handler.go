package auth

import (
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"

	"microservicio/internal/httpx"
)

// Handler expone registro, login y perfil por HTTP.
type Handler struct {
	svc    *Service
	tokens *TokenManager
	logger *slog.Logger
}

func NewHandler(svc *Service, tokens *TokenManager, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, tokens: tokens, logger: logger}
}

// RegisterRoutes registra las rutas /auth en el mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", h.register)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.Handle("GET /auth/me", h.tokens.RequireAuth(http.HandlerFunc(h.me)))
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var c Credentials
	if !httpx.DecodeJSON(w, r, &c) {
		return
	}
	u, err := h.svc.Register(r.Context(), c)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, u)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var c Credentials
	if !httpx.DecodeJSON(w, r, &c) {
		return
	}
	tok, err := h.svc.Login(r.Context(), c, clientIP(r))
	if err != nil {
		h.writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, tok)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFrom(r.Context())
	u, err := h.svc.Get(r.Context(), claims.Subject)
	if errors.Is(err, ErrNotFound) {
		// El token es válido pero la cuenta ya no existe.
		unauthorized(w, ErrInvalidToken.Error())
		return
	}
	if err != nil {
		h.writeErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// clientIP devuelve la IP de la conexión. No usa X-Forwarded-For: cualquier
// cliente puede falsearla, así que solo sería fiable detrás de un proxy conocido.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) writeErr(w http.ResponseWriter, err error) {
	if tooMany, ok := errors.AsType[*TooManyAttemptsError](err); ok {
		secs := int(math.Ceil(tooMany.RetryAfter.Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		httpx.WriteError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	switch {
	case errors.Is(err, ErrInvalid):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		unauthorized(w, err.Error())
	default:
		h.logger.Error("unexpected error", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
