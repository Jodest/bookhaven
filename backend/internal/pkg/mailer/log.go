package mailer

import (
	"context"

	"go.uber.org/zap"
)

// LogSender "delivers" email by writing it to the log. Used when SMTP is
// not configured. The text body is logged only when logBodies is true
// (non-production): bodies will contain tokens and must never reach
// production logs.
type LogSender struct {
	logger    *zap.Logger
	logBodies bool
}

// NewLogSender builds a LogSender.
func NewLogSender(logger *zap.Logger, logBodies bool) *LogSender {
	return &LogSender{logger: logger, logBodies: logBodies}
}

// Send logs the message and always returns nil.
func (s *LogSender) Send(_ context.Context, msg Message) error {
	fields := []zap.Field{
		zap.String("to", msg.To),
		zap.String("subject", msg.Subject),
	}
	if s.logBodies {
		fields = append(fields, zap.String("text_body", msg.TextBody))
	}
	s.logger.Info("mailer: email not sent (SMTP not configured)", fields...)
	return nil
}
