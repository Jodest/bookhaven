package mailer

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func validMessage() Message {
	return Message{
		To:       "reader@example.com",
		Subject:  "Подтвердите адрес почты",
		TextBody: "Привет! Ссылка: https://bookhaven.example.com/verify?token=abc",
	}
}

func TestMessageValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *Message)
		wantErr error
	}{
		{"valid", func(m *Message) {}, nil},
		{"valid with display name and html", func(m *Message) {
			m.To = "Иван <ivan@example.com>"
			m.HTMLBody = "<p>Привет</p>"
		}, nil},
		{"empty to", func(m *Message) { m.To = "" }, ErrInvalidRecipient},
		{"invalid to", func(m *Message) { m.To = "not-an-address" }, ErrInvalidRecipient},
		{"empty subject", func(m *Message) { m.Subject = "  " }, ErrEmptySubject},
		{"empty text body", func(m *Message) { m.TextBody = "" }, ErrEmptyTextBody},
		{"CRLF in to", func(m *Message) { m.To = "a@example.com\r\nBcc: evil@example.com" }, ErrHeaderInjection},
		{"LF in subject", func(m *Message) { m.Subject = "Hi\nBcc: evil@example.com" }, ErrHeaderInjection},
		{"CR in subject", func(m *Message) { m.Subject = "Hi\rthere" }, ErrHeaderInjection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMessage()
			tt.mutate(&m)
			err := m.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewSender(t *testing.T) {
	logger := zap.NewNop()

	if _, ok := NewSender(&config.Config{Env: "development"}, logger).(*LogSender); !ok {
		t.Error("empty SMTPHost: expected *LogSender")
	}

	cfg := &config.Config{Env: "production", SMTPHost: "smtp.example.com", SMTPPort: 587, MailFrom: "BookHaven <noreply@example.com>"}
	if _, ok := NewSender(cfg, logger).(*SMTPSender); !ok {
		t.Error("SMTPHost set: expected *SMTPSender")
	}
}

func TestNewSender_ProductionWithoutSMTPWarnsAndHidesBodies(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	sender := NewSender(&config.Config{Env: "production"}, zap.New(core))

	if logs.FilterLevelExact(zapcore.WarnLevel).Len() != 1 {
		t.Fatalf("expected one warning about missing SMTP, got %v", logs.All())
	}
	ls, ok := sender.(*LogSender)
	if !ok {
		t.Fatal("expected *LogSender")
	}
	if ls.logBodies {
		t.Error("bodies must not be logged in production")
	}
}

func TestNewSender_NeverLogsPassword(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	cfg := &config.Config{SMTPHost: "smtp.example.com", SMTPPort: 465, SMTPUser: "u", SMTPPassword: "super-secret", MailFrom: "a@example.com"}
	NewSender(cfg, zap.New(core))

	for _, entry := range logs.All() {
		for k, v := range entry.ContextMap() {
			if s, ok := v.(string); ok && s == "super-secret" {
				t.Fatalf("password logged in field %q", k)
			}
		}
	}
}

func TestLogSender(t *testing.T) {
	for _, logBodies := range []bool{true, false} {
		core, logs := observer.New(zapcore.InfoLevel)
		s := NewLogSender(zap.New(core), logBodies)

		if err := s.Send(context.Background(), validMessage()); err != nil {
			t.Fatalf("Send returned error: %v", err)
		}
		entries := logs.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}
		fields := entries[0].ContextMap()
		if fields["to"] != "reader@example.com" || fields["subject"] != validMessage().Subject {
			t.Errorf("unexpected fields: %v", fields)
		}
		_, hasBody := fields["text_body"]
		if hasBody != logBodies {
			t.Errorf("logBodies=%v: text_body present = %v", logBodies, hasBody)
		}
	}
}
