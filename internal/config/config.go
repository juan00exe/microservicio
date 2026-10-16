// Package config carga la configuración del servicio desde variables de entorno.
package config

import (
	"fmt"
	"os"
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
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
