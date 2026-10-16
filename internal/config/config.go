// Package config carga la configuración del servicio desde variables de entorno.
package config

import "os"

// Config contiene la configuración del servicio.
type Config struct {
	Port string
	// DatabaseURL es la cadena de conexión a PostgreSQL. Si está vacía,
	// el servicio usa almacenamiento en memoria.
	DatabaseURL string
}

// Load lee la configuración desde el entorno aplicando valores por defecto.
func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
