// Package mailer is BookHaven's shared transactional-email infrastructure.
//
// Flow: a service in the api process renders a Message (subject, text and
// optional HTML body) and calls Enqueuer.Enqueue. Queue turns it into an
// asynq task of type TaskTypeSendEmail on the dedicated QueueName queue.
// cmd/worker runs a separate asynq server for that queue whose Handler
// passes the decoded Message to a Sender: SMTPSender when SMTP_HOST is
// configured, LogSender otherwise (local development without Mailpit, or a
// production deploy where SMTP has not been set up yet). No request
// handler ever talks to SMTP directly.
//
// Delivery semantics are at-least-once: asynq may retry a task whose send
// actually succeeded (e.g. the connection dropped after DATA was
// accepted), so callers must design emails to be harmless when duplicated.
// Transient failures (network, timeouts, SMTP 4xx) are retried up to
// MaxRetry(8) with asynq's default backoff; permanent SMTP rejections
// (5xx) and malformed/invalid payloads are not retried (asynq.SkipRetry).
// Completed tasks are not retained in Redis, because future payloads carry
// tokens (verification / password-reset links).
//
// Rendering happens in the caller at enqueue time; the worker sends
// exactly what is in the payload and knows nothing about email types.
//
// Template convention (implemented in render.go): templates/ under this
// package embedded via //go:embed, with a shared layout.html.tmpl and
// per-email pairs <name>.html.tmpl + <name>.txt.tmpl, rendered by
// Render(name, data) using html/template for
// the HTML part (auto-escaping user-provided values) and text/template for
// the text part. The subject lives in the template set (e.g. a
// {{define "subject"}} block in the txt template). Email copy is Russian;
// absolute links are built from a config value (e.g. WEB_BASE_URL), never
// hardcoded.
package mailer

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/bookhaven/backend/internal/config"
	"go.uber.org/zap"
)

// Message is one fully-rendered transactional email to a single recipient.
type Message struct {
	To       string `json:"to"`
	Subject  string `json:"subject"`
	TextBody string `json:"text_body"`           // required
	HTMLBody string `json:"html_body,omitempty"` // optional
}

// Validation errors returned by Message.Validate.
var (
	ErrInvalidRecipient = errors.New("mailer: invalid recipient address")
	ErrEmptySubject     = errors.New("mailer: subject is required")
	ErrEmptyTextBody    = errors.New("mailer: text body is required")
	ErrHeaderInjection  = errors.New("mailer: CR/LF not allowed in recipient or subject")
)

// Validate checks that the message can be safely turned into an email: a
// single parseable recipient, a non-empty subject and text body, and no
// CR/LF in header-bound fields (header injection).
func (m Message) Validate() error {
	if strings.ContainsAny(m.To, "\r\n") || strings.ContainsAny(m.Subject, "\r\n") {
		return ErrHeaderInjection
	}
	if strings.TrimSpace(m.To) == "" {
		return ErrInvalidRecipient
	}
	if _, err := mail.ParseAddress(m.To); err != nil {
		return ErrInvalidRecipient
	}
	if strings.TrimSpace(m.Subject) == "" {
		return ErrEmptySubject
	}
	if strings.TrimSpace(m.TextBody) == "" {
		return ErrEmptyTextBody
	}
	return nil
}

// Sender delivers a single Message.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// NewSender returns an SMTPSender when cfg.SMTPHost is set, otherwise a
// LogSender. It logs which sender is active (never the password); in
// production an unconfigured SMTP host only produces a warning.
func NewSender(cfg *config.Config, logger *zap.Logger) Sender {
	if cfg.SMTPHost != "" {
		logger.Info("mailer: using SMTP sender",
			zap.String("addr", fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)),
			zap.String("from", cfg.MailFrom),
			zap.Bool("auth", cfg.SMTPUser != ""),
		)
		return NewSMTPSender(SMTPOptions{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.MailFrom,
		})
	}

	isProduction := cfg.Env == "production"
	if isProduction {
		logger.Warn("SMTP not configured, emails will only be logged")
	} else {
		logger.Info("mailer: SMTP not configured, using log sender")
	}
	return NewLogSender(logger, !isProduction)
}
