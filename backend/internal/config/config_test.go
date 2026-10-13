package config

import "testing"

// setSMTPEnv clears every SMTP-related variable so the host environment
// (or a stray .env) cannot leak into a test case.
func setSMTPEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "MAIL_FROM", "APP_ENV", "JWT_SECRET", "WEB_BASE_URL"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_SMTPDefaults(t *testing.T) {
	setSMTPEnv(t, nil)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SMTPHost != "" || cfg.SMTPUser != "" || cfg.SMTPPassword != "" {
		t.Errorf("expected empty SMTP host/user/password, got %q/%q/%q", cfg.SMTPHost, cfg.SMTPUser, cfg.SMTPPassword)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want 587", cfg.SMTPPort)
	}
	if cfg.MailFrom != defaultMailFrom {
		t.Errorf("MailFrom = %q, want %q", cfg.MailFrom, defaultMailFrom)
	}
}

func TestLoad_SMTPConfigured(t *testing.T) {
	setSMTPEnv(t, map[string]string{
		"SMTP_HOST":     "smtp.example.com",
		"SMTP_PORT":     "465",
		"SMTP_USER":     "user",
		"SMTP_PASSWORD": "secret",
		"MAIL_FROM":     "BookHaven <noreply@example.com>",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != 465 || cfg.SMTPUser != "user" || cfg.SMTPPassword != "secret" {
		t.Errorf("unexpected SMTP config: %+v", cfg)
	}
}

func TestLoad_InvalidMailFromWithHost(t *testing.T) {
	setSMTPEnv(t, map[string]string{
		"SMTP_HOST": "smtp.example.com",
		"MAIL_FROM": "not an address",
	})

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid MAIL_FROM with SMTP_HOST set")
	}
}

func TestLoad_ProductionRequiresExplicitMailFrom(t *testing.T) {
	setSMTPEnv(t, map[string]string{
		"APP_ENV":    "production",
		"JWT_SECRET": "a-real-secret",
		"SMTP_HOST":  "smtp.example.com",
	})

	if _, err := Load(); err == nil {
		t.Fatal("expected error for missing MAIL_FROM with SMTP_HOST set in production")
	}
}

func TestLoad_InvalidMailFromWithoutHostIgnored(t *testing.T) {
	setSMTPEnv(t, map[string]string{"MAIL_FROM": "not an address"})

	if _, err := Load(); err != nil {
		t.Fatalf("expected no error when SMTP_HOST is empty, got %v", err)
	}
}

func TestLoad_NonNumericSMTPPort(t *testing.T) {
	setSMTPEnv(t, map[string]string{"SMTP_PORT": "abc"})

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric SMTP_PORT")
	}
}

func TestLoad_WebBaseURLDefault(t *testing.T) {
	setSMTPEnv(t, nil)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebBaseURL != "http://localhost:3000" {
		t.Errorf("WebBaseURL = %q, want http://localhost:3000", cfg.WebBaseURL)
	}
}

func TestLoad_WebBaseURLTrailingSlashTrimmed(t *testing.T) {
	setSMTPEnv(t, map[string]string{"WEB_BASE_URL": "https://bookhaven.example.com/"})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebBaseURL != "https://bookhaven.example.com" {
		t.Errorf("WebBaseURL = %q, want https://bookhaven.example.com", cfg.WebBaseURL)
	}
}

func TestLoad_WebBaseURLInvalid(t *testing.T) {
	for _, value := range []string{"bookhaven.example.com", "ftp://bookhaven.example.com", "https://", "/verify"} {
		t.Run(value, func(t *testing.T) {
			setSMTPEnv(t, map[string]string{"WEB_BASE_URL": value})
			if _, err := Load(); err == nil {
				t.Fatalf("expected error for WEB_BASE_URL=%q", value)
			}
		})
	}
}

func TestLoad_ProductionRequiresExplicitWebBaseURL(t *testing.T) {
	setSMTPEnv(t, map[string]string{
		"APP_ENV":    "production",
		"JWT_SECRET": "a-real-secret",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error for missing WEB_BASE_URL in production")
	}

	t.Setenv("WEB_BASE_URL", "https://bookhaven.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebBaseURL != "https://bookhaven.example.com" {
		t.Errorf("WebBaseURL = %q", cfg.WebBaseURL)
	}
}

func setHarvestScheduleEnv(t *testing.T, env map[string]string) {
	t.Helper()
	setSMTPEnv(t, nil)
	for _, k := range []string{"HARVEST_SCHEDULE_ENABLED", "HARVEST_SCHEDULE_CRON", "HARVEST_SCHEDULE_BATCH"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_HarvestScheduleDefaults(t *testing.T) {
	setHarvestScheduleEnv(t, nil)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HarvestScheduleEnabled {
		t.Error("scheduled harvest must be off by default")
	}
	if cfg.HarvestScheduleCron != "0 4 * * *" {
		t.Errorf("HarvestScheduleCron = %q, want 0 4 * * *", cfg.HarvestScheduleCron)
	}
	if cfg.HarvestScheduleBatch != 10 {
		t.Errorf("HarvestScheduleBatch = %d, want 10", cfg.HarvestScheduleBatch)
	}
}

func TestLoad_HarvestScheduleConfigured(t *testing.T) {
	setHarvestScheduleEnv(t, map[string]string{
		"HARVEST_SCHEDULE_ENABLED": "true",
		"HARVEST_SCHEDULE_CRON":    " 15 5 * * 1 ",
		"HARVEST_SCHEDULE_BATCH":   "25",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.HarvestScheduleEnabled || cfg.HarvestScheduleCron != "15 5 * * 1" || cfg.HarvestScheduleBatch != 25 {
		t.Errorf("unexpected schedule config: %v %q %d", cfg.HarvestScheduleEnabled, cfg.HarvestScheduleCron, cfg.HarvestScheduleBatch)
	}
}

func TestLoad_HarvestScheduleParsing(t *testing.T) {
	cases := []struct {
		enabled     string
		batch       string
		wantEnabled bool
		wantBatch   int
	}{
		{enabled: "yes", batch: "abc", wantEnabled: false, wantBatch: 10},
		{enabled: "1", batch: "0", wantEnabled: true, wantBatch: 1},
		{enabled: "false", batch: "-5", wantEnabled: false, wantBatch: 1},
		{enabled: "TRUE", batch: "51", wantEnabled: true, wantBatch: 50},
		{enabled: "", batch: "1000", wantEnabled: false, wantBatch: 50},
		{enabled: "", batch: "50", wantEnabled: false, wantBatch: 50},
	}
	for _, tc := range cases {
		setHarvestScheduleEnv(t, map[string]string{"HARVEST_SCHEDULE_ENABLED": tc.enabled, "HARVEST_SCHEDULE_BATCH": tc.batch})
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if cfg.HarvestScheduleEnabled != tc.wantEnabled || cfg.HarvestScheduleBatch != tc.wantBatch {
			t.Errorf("enabled=%q batch=%q: got %v %d, want %v %d", tc.enabled, tc.batch, cfg.HarvestScheduleEnabled, cfg.HarvestScheduleBatch, tc.wantEnabled, tc.wantBatch)
		}
	}
}

func setAIModerationEnv(t *testing.T, env map[string]string) {
	t.Helper()
	setSMTPEnv(t, nil)
	for _, k := range []string{"AI_MODERATION_ENABLED", "ANTHROPIC_API_KEY", "AI_MODERATION_MODEL", "AI_MODERATION_CRON", "AI_MODERATION_BATCH"} {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_AIModerationDefaults(t *testing.T) {
	setAIModerationEnv(t, nil)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AIModerationEnabled || cfg.AIModerationActive() {
		t.Error("AI moderation must be off by default")
	}
	if cfg.AIModerationModel != "claude-haiku-5-5" || cfg.AIModerationCron != "*/30 * * * *" || cfg.AIModerationBatch != 50 {
		t.Errorf("unexpected defaults: %q %q %d", cfg.AIModerationModel, cfg.AIModerationCron, cfg.AIModerationBatch)
	}
}

func TestLoad_AIModerationActiveAndParsing(t *testing.T) {
	cases := []struct {
		enabled, key, batch string
		wantActive          bool
		wantBatch           int
	}{
		{enabled: "true", key: "sk-test", batch: "10", wantActive: true, wantBatch: 10},
		{enabled: "true", key: "  ", batch: "abc", wantActive: false, wantBatch: 50},
		{enabled: "false", key: "sk-test", batch: "0", wantActive: false, wantBatch: 1},
		{enabled: "", key: "sk-test", batch: "201", wantActive: false, wantBatch: 200},
		{enabled: "1", key: "sk-test", batch: "200", wantActive: true, wantBatch: 200},
	}
	for _, tc := range cases {
		setAIModerationEnv(t, map[string]string{"AI_MODERATION_ENABLED": tc.enabled, "ANTHROPIC_API_KEY": tc.key, "AI_MODERATION_BATCH": tc.batch})
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if cfg.AIModerationActive() != tc.wantActive || cfg.AIModerationBatch != tc.wantBatch {
			t.Errorf("enabled=%q key=%q batch=%q: got active=%v batch=%d", tc.enabled, tc.key, tc.batch, cfg.AIModerationActive(), cfg.AIModerationBatch)
		}
	}

	setAIModerationEnv(t, map[string]string{"AI_MODERATION_MODEL": " claude-sonnet-5-5 ", "AI_MODERATION_CRON": " 0 * * * * "})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AIModerationModel != "claude-sonnet-5-5" || cfg.AIModerationCron != "0 * * * *" {
		t.Errorf("unexpected model/cron: %q %q", cfg.AIModerationModel, cfg.AIModerationCron)
	}
}
