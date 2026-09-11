package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	Env            string
	Port           string
	DatabaseURL    string
	RedisURL       string
	LogLevel       string
	JWTSecret      string
	JWTExpiresIn   string
	AllowedOrigins []string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:            getEnv("APP_ENV", "development"),
		Port:           getEnv("APP_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://bookhaven:bookhaven@localhost:5432/bookhaven?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "localhost:6379"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiresIn:   getEnv("JWT_EXPIRES_IN", "24h"),
		AllowedOrigins: splitEnv(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001")),
	}

	if cfg.Env == "production" && cfg.JWTSecret == "change-me-in-production" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func splitEnv(value string) []string {
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}
