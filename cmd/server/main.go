package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"microservicio/internal/auth"
	"microservicio/internal/config"
	"microservicio/internal/database"
	"microservicio/internal/docs"
	"microservicio/internal/httpx"
	"microservicio/internal/product"
)

func main() {
	logger := slog.New(httpx.NewContextHandler(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(logger); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		repo     product.Repository
		userRepo auth.Repository
		pool     *pgxpool.Pool
	)
	if cfg.DatabaseURL != "" {
		pool, err = database.Connect(ctx, cfg.DatabaseURL, logger)
		if err != nil {
			return err
		}
		defer pool.Close()
		if err := database.Migrate(ctx, pool, logger); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		repo = product.NewPostgresRepository(pool)
		userRepo = auth.NewPostgresRepository(pool)
		logger.Info("using postgres storage")
	} else {
		repo = product.NewMemoryRepository()
		userRepo = auth.NewMemoryRepository()
		logger.Warn("DATABASE_URL not set, using in-memory storage")
	}

	tokens, err := newTokenManager(cfg, logger)
	if err != nil {
		return err
	}
	authSvc := auth.NewService(userRepo, tokens, auth.LoginLimits{
		MaxPerEmail: cfg.LoginMaxPerEmail,
		MaxPerIP:    cfg.LoginMaxPerIP,
		Lockout:     cfg.LoginLockout,
	})
	if err := ensureAdmin(ctx, authSvc, cfg, logger); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if pool != nil {
			pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := pool.Ping(pingCtx); err != nil {
				httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "down"})
				return
			}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	auth.NewHandler(authSvc, tokens, logger).RegisterRoutes(mux)
	svc := product.NewService(repo)
	product.NewHandler(svc, logger).RegisterRoutes(mux, tokens.RequireRole(auth.RoleAdmin))
	docs.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.RequestID(httpx.Recover(logger, httpx.Logging(logger, mux))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// newTokenManager usa JWT_SECRET o, si no está definido, un secreto aleatorio
// (válido solo mientras el proceso siga vivo).
func newTokenManager(cfg config.Config, logger *slog.Logger) (*auth.TokenManager, error) {
	secret := []byte(cfg.JWTSecret)
	if len(secret) == 0 {
		secret = make([]byte, auth.MinSecretLen)
		_, _ = rand.Read(secret)
		logger.Warn("JWT_SECRET not set, using a random secret: tokens will stop working on restart")
	}
	tm, err := auth.NewTokenManager(secret, cfg.JWTTTL)
	if err != nil {
		return nil, fmt.Errorf("JWT_SECRET: %w", err)
	}
	return tm, nil
}

// ensureAdmin crea la cuenta admin de ADMIN_EMAIL/ADMIN_PASSWORD si no existe.
func ensureAdmin(ctx context.Context, svc *auth.Service, cfg config.Config, logger *slog.Logger) error {
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		logger.Warn("ADMIN_EMAIL/ADMIN_PASSWORD not set, no admin account will be created")
		return nil
	}
	u, created, err := svc.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	switch {
	case created:
		logger.Info("admin account created", "email", u.Email)
	case u.Role != auth.RoleAdmin:
		logger.Warn("ADMIN_EMAIL belongs to an existing non-admin account; it was not promoted", "email", u.Email)
	}
	return nil
}
