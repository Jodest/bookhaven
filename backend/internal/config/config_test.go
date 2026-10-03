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
