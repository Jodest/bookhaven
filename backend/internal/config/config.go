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

	// Scheduled harvest of flagged authors (worker only; see
	// internal/modules/harvest/service ScheduleTick). Off unless
	// HARVEST_SCHEDULE_ENABLED is true. HarvestScheduleCron is a UTC cron
	// spec; HarvestScheduleBatch is the max authors per run (1..50).
	HarvestScheduleEnabled bool
	HarvestScheduleCron    string
	HarvestScheduleBatch   int

	// Scheduled catalog-wide adaptation sync (worker only; see
	// internal/modules/screenworks/service CatalogSyncTick). Off unless
	// ADAPTATION_SYNC_SCHEDULE_ENABLED is true. AdaptationSyncScheduleCron is
	// a UTC cron spec (weekly by default).
	AdaptationSyncScheduleEnabled bool
	AdaptationSyncScheduleCron    string

	// Nightly recompute of automatic curated collections (worker only; see
	// internal/modules/curated/service). On by default: only an explicit
	// falsy AUTO_COLLECTIONS_SCHEDULE_ENABLED turns it off. The cron is UTC.
	AutoCollectionsScheduleEnabled bool
	AutoCollectionsScheduleCron    string

	// AI pre-moderation of pending books (worker computes verdicts, API
	// exposes trigger/status; see internal/modules/aimoderation). Active
	// only when AI_MODERATION_ENABLED is true and ANTHROPIC_API_KEY is set
	// (AIModerationActive); both api and worker need both. The key is a
	// secret: never log it. AIModerationModel is the exact model id;
	// AIModerationCron is a UTC cron spec; AIModerationBatch is the max
	// books per sweep (1..200).
	AIModerationEnabled bool
	AnthropicAPIKey     string
	AIModerationModel   string
	AIModerationCron    string
	AIModerationBatch   int

	// AI curator of author import lists (worker asks the model for author
	// names and resolves them against Wikidata; API exposes the requests;
	// see internal/modules/aicurator). Active only when AI_CURATOR_ENABLED
	// is true and ANTHROPIC_API_KEY (shared with AI moderation) is set
	// (AICuratorActive); both api and worker need both. AICuratorModel is
	// the exact model id.
	AICuratorEnabled bool
	AICuratorModel   string

	// AI-generated book description drafts (worker generates them, API
	// exposes trigger/review; see internal/modules/aidescriptions). Active
	// only when AI_DESCRIPTIONS_ENABLED is true and ANTHROPIC_API_KEY
	// (shared with AI moderation) is set (AIDescriptionsActive); both api
	// and worker need both. AIDescriptionsModel is the exact model id.
	AIDescriptionsEnabled bool
	AIDescriptionsModel   string

	// AI collection drafts (an admin asks for a ranked draft of a manual
	// collection; see internal/modules/aicollections). Active only when
	// AI_COLLECTIONS_ENABLED is true and ANTHROPIC_API_KEY is set
	// (AICollectionsActive); both api and worker need both.
	AICollectionsEnabled bool
	AICollectionsModel   string

	// AI genre suggestions (a background analysis proposes genre merges and
	// rejections to the admin; see internal/modules/aigenres). Active only
	// when AI_GENRES_ENABLED is true and ANTHROPIC_API_KEY is set
	// (AIGenresActive); both api and worker need both. AIGenresCron is a UTC
	// cron spec (empty = no schedule); AIGenresBatch is the number of
	// non-anchor genres per model request.
	AIGenresEnabled bool
	AIGenresModel   string
	AIGenresCron    string
	AIGenresBatch   int
}

// AIGenresActive reports whether AI genre suggestions are switched on and
// have an API key.
func (c *Config) AIGenresActive() bool {
	return c.AIGenresEnabled && strings.TrimSpace(c.AnthropicAPIKey) != ""
}

// AICollectionsActive reports whether AI collection drafts are switched on
// and have an API key.
func (c *Config) AICollectionsActive() bool {
	return c.AICollectionsEnabled && strings.TrimSpace(c.AnthropicAPIKey) != ""
}

// AIDescriptionsActive reports whether AI descriptions are switched on and
// have an API key.
func (c *Config) AIDescriptionsActive() bool {
	return c.AIDescriptionsEnabled && strings.TrimSpace(c.AnthropicAPIKey) != ""
}

// AICuratorActive reports whether AI curation is switched on and has an
// API key.
func (c *Config) AICuratorActive() bool {
	return c.AICuratorEnabled && strings.TrimSpace(c.AnthropicAPIKey) != ""
}

// AIModerationActive reports whether AI moderation is switched on and has
// an API key.
func (c *Config) AIModerationActive() bool {
	return c.AIModerationEnabled && strings.TrimSpace(c.AnthropicAPIKey) != ""
}

// Scheduled harvest defaults and bounds.
const (
	DefaultHarvestScheduleCron  = "0 4 * * *"
	DefaultHarvestScheduleBatch = 10
	MaxHarvestScheduleBatch     = 50
)

// DefaultAdaptationSyncScheduleCron is Sunday 05:00 UTC.
const DefaultAdaptationSyncScheduleCron = "0 5 * * 0"

// DefaultAutoCollectionsScheduleCron is 01:00 UTC (04:00 MSK), before the
// 03:30 UTC similarity job.
const DefaultAutoCollectionsScheduleCron = "0 1 * * *"

// AI moderation defaults and bounds.
const (
	DefaultAIModerationModel = "claude-haiku-5-5"
	DefaultAIModerationCron  = "*/30 * * * *"
	DefaultAIModerationBatch = 50
	MaxAIModerationBatch     = 200
)

// DefaultAICuratorModel is the AI curator's default model id.
const DefaultAICuratorModel = "claude-haiku-5-5"

// DefaultAIDescriptionsModel is the AI descriptions' default model id
// (Sonnet: the only generative task, where world knowledge matters).
const DefaultAIDescriptionsModel = "claude-sonnet-5-5"

// DefaultAICollectionsModel is the AI collection drafts' default model id
// (Sonnet: recall of real, correctly attributed works is the whole value).
const DefaultAICollectionsModel = "claude-sonnet-5-5"

// AI genre suggestions defaults: Sonnet (judging whether two names denote one
// genre needs language knowledge), weekly on Monday 03:00 UTC.
const (
	DefaultAIGenresModel = "claude-sonnet-5-5"
	DefaultAIGenresCron  = "0 3 * * 1"
	DefaultAIGenresBatch = 300
	MaxAIGenresBatch     = 500
)

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

	cfg.HarvestScheduleEnabled = parseBoolEnv("HARVEST_SCHEDULE_ENABLED")
	cfg.HarvestScheduleCron = strings.TrimSpace(getEnv("HARVEST_SCHEDULE_CRON", DefaultHarvestScheduleCron))
	cfg.HarvestScheduleBatch = parseHarvestScheduleBatch(getEnv("HARVEST_SCHEDULE_BATCH", ""))

	cfg.AdaptationSyncScheduleEnabled = parseBoolEnv("ADAPTATION_SYNC_SCHEDULE_ENABLED")
	cfg.AdaptationSyncScheduleCron = strings.TrimSpace(getEnv("ADAPTATION_SYNC_SCHEDULE_CRON", DefaultAdaptationSyncScheduleCron))

	cfg.AutoCollectionsScheduleEnabled = parseBoolEnvDefaultTrue("AUTO_COLLECTIONS_SCHEDULE_ENABLED")
	cfg.AutoCollectionsScheduleCron = strings.TrimSpace(getEnv("AUTO_COLLECTIONS_SCHEDULE_CRON", DefaultAutoCollectionsScheduleCron))

	cfg.AIModerationEnabled = parseBoolEnv("AI_MODERATION_ENABLED")
	cfg.AnthropicAPIKey = strings.TrimSpace(getEnv("ANTHROPIC_API_KEY", ""))
	cfg.AIModerationModel = strings.TrimSpace(getEnv("AI_MODERATION_MODEL", DefaultAIModerationModel))
	if cfg.AIModerationModel == "" {
		cfg.AIModerationModel = DefaultAIModerationModel
	}
	cfg.AIModerationCron = strings.TrimSpace(getEnv("AI_MODERATION_CRON", DefaultAIModerationCron))
	cfg.AIModerationBatch = parseClampedInt(getEnv("AI_MODERATION_BATCH", ""), DefaultAIModerationBatch, MaxAIModerationBatch)

	cfg.AICuratorEnabled = parseBoolEnv("AI_CURATOR_ENABLED")
	cfg.AICuratorModel = strings.TrimSpace(getEnv("AI_CURATOR_MODEL", DefaultAICuratorModel))
	if cfg.AICuratorModel == "" {
		cfg.AICuratorModel = DefaultAICuratorModel
	}

	cfg.AIDescriptionsEnabled = parseBoolEnv("AI_DESCRIPTIONS_ENABLED")
	cfg.AIDescriptionsModel = strings.TrimSpace(getEnv("AI_DESCRIPTIONS_MODEL", DefaultAIDescriptionsModel))
	if cfg.AIDescriptionsModel == "" {
		cfg.AIDescriptionsModel = DefaultAIDescriptionsModel
	}

	cfg.AICollectionsEnabled = parseBoolEnv("AI_COLLECTIONS_ENABLED")
	cfg.AICollectionsModel = strings.TrimSpace(getEnv("AI_COLLECTIONS_MODEL", DefaultAICollectionsModel))
	if cfg.AICollectionsModel == "" {
		cfg.AICollectionsModel = DefaultAICollectionsModel
	}

	cfg.AIGenresEnabled = parseBoolEnv("AI_GENRES_ENABLED")
	cfg.AIGenresModel = strings.TrimSpace(getEnv("AI_GENRES_MODEL", DefaultAIGenresModel))
	if cfg.AIGenresModel == "" {
		cfg.AIGenresModel = DefaultAIGenresModel
	}
	// Unlike the other crons, an explicitly empty AI_GENRES_CRON disables the
	// schedule (the analysis then runs only on demand).
	if raw, ok := os.LookupEnv("AI_GENRES_CRON"); ok {
		cfg.AIGenresCron = strings.TrimSpace(raw)
	} else {
		cfg.AIGenresCron = DefaultAIGenresCron
	}
	cfg.AIGenresBatch = parseClampedInt(getEnv("AI_GENRES_BATCH", ""), DefaultAIGenresBatch, MaxAIGenresBatch)

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

// parseBoolEnv reads a boolean env var; unset or unparsable means false.
func parseBoolEnv(key string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	return err == nil && v
}

// parseBoolEnvDefaultTrue reads a boolean env var; unset or unparsable means
// true, only an explicit falsy value gives false.
func parseBoolEnvDefaultTrue(key string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	return err != nil || v
}

// parseHarvestScheduleBatch parses HARVEST_SCHEDULE_BATCH: a non-integer
// (or unset) value is the default, anything else is clamped to
// 1..MaxHarvestScheduleBatch.
func parseHarvestScheduleBatch(raw string) int {
	return parseClampedInt(raw, DefaultHarvestScheduleBatch, MaxHarvestScheduleBatch)
}

// parseClampedInt parses a batch-size env value: a non-integer (or unset)
// value is def, anything else is clamped to 1..max.
func parseClampedInt(raw string, def, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	if n < 1 {
		return 1
	}
	if n > max {
		return max
	}
	return n
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
