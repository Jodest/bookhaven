package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	Env          string
	Port         string
	DatabaseURL  string
	RedisURL     string
	LogLevel     string
	JWTSecret    string
	JWTExpiresIn string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:          getEnv("APP_ENV", "development"),
		Port:         getEnv("APP_PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://bookhaven:bookhaven@localhost:5432/bookhaven?sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "localhost:6379"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiresIn: getEnv("JWT_EXPIRES_IN", "24h"),
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
