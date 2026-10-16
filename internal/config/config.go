// Package config carga la configuración del servicio desde variables de entorno.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contiene la configuración del servicio.
type Config struct {
	Port string
	// DatabaseURL es la cadena de conexión a PostgreSQL. Si está vacía,
	// el servicio usa almacenamiento en memoria.
	DatabaseURL string

	// JWTSecret firma los tokens (mínimo 32 bytes). Si está vacío se genera uno
	// aleatorio al arrancar y los tokens dejan de valer al reiniciar.
	JWTSecret string
	JWTTTL    time.Duration

	// AdminEmail y AdminPassword, si ambos están definidos, crean la cuenta
	// admin al arrancar cuando todavía no existe.
	AdminEmail    string
	AdminPassword string

	// Bloqueo de login por intentos fallidos (0 desactiva ese límite).
	LoginMaxPerEmail int
	LoginMaxPerIP    int
	LoginLockout     time.Duration
}

// Load lee la configuración desde el entorno aplicando valores por defecto.
func Load() (Config, error) {
	cfg := Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		JWTSecret:     getEnv("JWT_SECRET", ""),
		AdminEmail:    getEnv("ADMIN_EMAIL", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
	}
	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "1h"))
	if err != nil || ttl <= 0 {
		return Config{}, fmt.Errorf("JWT_TTL must be a positive duration like 15m or 1h")
	}
	cfg.JWTTTL = ttl

	if cfg.LoginMaxPerEmail, err = intEnv("LOGIN_MAX_ATTEMPTS", 5); err != nil {
		return Config{}, err
	}
	if cfg.LoginMaxPerIP, err = intEnv("LOGIN_MAX_ATTEMPTS_PER_IP", 20); err != nil {
		return Config{}, err
	}
	lockout, err := time.ParseDuration(getEnv("LOGIN_LOCKOUT", "15m"))
	if err != nil || lockout <= 0 {
		return Config{}, fmt.Errorf("LOGIN_LOCKOUT must be a positive duration like 15m")
	}
	cfg.LoginLockout = lockout
	return cfg, nil
}

func intEnv(key string, fallback int) (int, error) {
	v := getEnv(key, "")
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return n, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
