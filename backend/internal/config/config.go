package config

import (
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
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

	// Outbound email (see internal/pkg/mailer). SMTPHost empty means
	// "SMTP not configured": the worker only logs emails instead of
	// sending them. SMTPPassword must never be logged.
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	MailFrom     string

	// WebBaseURL is the public origin of the web app (no trailing slash),
	// used to build absolute links in emails (e.g. /verify-email).
	WebBaseURL string
}

// defaultMailFrom is used when MAIL_FROM is unset (dev / Mailpit).
const defaultMailFrom = "BookHaven <noreply@bookhaven.local>"

// defaultWebBaseURL is used when WEB_BASE_URL is unset (dev).
const defaultWebBaseURL = "http://localhost:3000"

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
		SMTPHost:       strings.TrimSpace(getEnv("SMTP_HOST", "")),
		SMTPUser:       getEnv("SMTP_USER", ""),
		SMTPPassword:   getEnv("SMTP_PASSWORD", ""),
		MailFrom:       getEnv("MAIL_FROM", defaultMailFrom),
		WebBaseURL:     strings.TrimRight(strings.TrimSpace(getEnv("WEB_BASE_URL", defaultWebBaseURL)), "/"),
	}

	smtpPort, err := strconv.Atoi(strings.TrimSpace(getEnv("SMTP_PORT", "587")))
	if err != nil || smtpPort <= 0 || smtpPort > 65535 {
		return nil, fmt.Errorf("SMTP_PORT must be a valid port number")
	}
	cfg.SMTPPort = smtpPort

	if cfg.SMTPHost != "" {
		// The dev default sender is not on a domain with SPF/DKIM, so a real
		// production SMTP setup must name its sender explicitly.
		if cfg.Env == "production" && strings.TrimSpace(os.Getenv("MAIL_FROM")) == "" {
			return nil, fmt.Errorf("MAIL_FROM must be set when SMTP_HOST is set in production")
		}
		if _, err := mail.ParseAddress(cfg.MailFrom); err != nil {
			return nil, fmt.Errorf("MAIL_FROM must be a valid email address when SMTP_HOST is set: %w", err)
		}
	}

	// Links in emails must point at the real public web origin in
	// production, never at the localhost dev default.
	if cfg.Env == "production" && strings.TrimSpace(os.Getenv("WEB_BASE_URL")) == "" {
		return nil, fmt.Errorf("WEB_BASE_URL must be set in production")
	}
	if u, err := url.Parse(cfg.WebBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("WEB_BASE_URL must be an absolute http(s) URL with a host")
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
