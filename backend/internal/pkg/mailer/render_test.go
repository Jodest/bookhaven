package mailer

import (
	"strings"
	"testing"
)

func TestRender_VerifyEmail(t *testing.T) {
	url := "http://localhost:3000/verify-email?token=abc123"
	msg, err := Render("verify_email", VerifyEmailData{Name: "Анна", VerifyURL: url, TTLHours: 24})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if msg.Subject != "Подтвердите адрес электронной почты — BookHaven" {
		t.Errorf("unexpected subject %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, url) {
		t.Errorf("text body does not contain URL: %q", msg.TextBody)
	}
	if !strings.Contains(msg.TextBody, "Здравствуйте, Анна!") {
		t.Errorf("text body does not greet by name: %q", msg.TextBody)
	}
	if !strings.Contains(msg.TextBody, "24 часа") {
		t.Errorf("text body does not mention TTL: %q", msg.TextBody)
	}
	if !strings.Contains(msg.HTMLBody, "Подтвердить email") {
		t.Errorf("html body has no button: %q", msg.HTMLBody)
	}
	// html/template escapes & in attribute/text context; the URL must still be present.
	if !strings.Contains(msg.HTMLBody, "http://localhost:3000/verify-email?token=abc123") {
		t.Errorf("html body does not contain URL: %q", msg.HTMLBody)
	}

	msg.To = "anna@example.com"
	if err := msg.Validate(); err != nil {
		t.Errorf("rendered message does not validate: %v", err)
	}
}

func TestRender_VerifyEmail_EmptyNameFallback(t *testing.T) {
	msg, err := Render("verify_email", VerifyEmailData{VerifyURL: "http://x/verify-email?token=t", TTLHours: 24})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(msg.TextBody, "Здравствуйте!") {
		t.Errorf("expected fallback greeting, got %q", msg.TextBody)
	}
}

func TestRender_VerifyEmail_EscapesNameInHTML(t *testing.T) {
	msg, err := Render("verify_email", VerifyEmailData{Name: "<script>alert(1)</script>", VerifyURL: "http://x/verify-email?token=t", TTLHours: 24})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(msg.HTMLBody, "<script>") {
		t.Errorf("html body contains unescaped script tag: %q", msg.HTMLBody)
	}
	if !strings.Contains(msg.HTMLBody, "&lt;script&gt;") {
		t.Errorf("expected escaped name in html body: %q", msg.HTMLBody)
	}
}

func TestRender_UnknownTemplate(t *testing.T) {
	if _, err := Render("does_not_exist", nil); err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestRender_AllTemplatesParse(t *testing.T) {
	set, err := loadTemplates()
	if err != nil {
		t.Fatalf("templates failed to parse: %v", err)
	}
	if _, ok := set["verify_email"]; !ok {
		t.Fatal("verify_email template not found")
	}
}
