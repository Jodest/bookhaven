package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/textproto"
	"time"

	"github.com/hibiken/asynq"
)

// TaskTypeSendEmail is the asynq task type carrying one rendered Message.
const TaskTypeSendEmail = "email:send"

// QueueName is the dedicated asynq queue email tasks run on (served by its
// own asynq.Server in cmd/worker so email is never stuck behind a
// long-running harvest task).
const QueueName = "email"

// sendEmailTaskOptions are the fixed options for TaskTypeSendEmail. No
// Retention: completed tasks are deleted immediately because payloads will
// contain tokens.
func sendEmailTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(8),
		asynq.Timeout(1 * time.Minute),
	}
}

// NewSendEmailTask validates msg and builds the email:send task.
func NewSendEmailTask(msg Message) (*asynq.Task, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("mailer: marshal payload: %w", err)
	}
	return asynq.NewTask(TaskTypeSendEmail, payload, sendEmailTaskOptions()...), nil
}

// Handler processes email:send tasks in the worker.
type Handler struct {
	sender Sender
}

// NewHandler builds a Handler delivering through sender.
func NewHandler(sender Sender) *Handler {
	return &Handler{sender: sender}
}

// HandleTask decodes and validates the payload and sends it. Malformed or
// invalid payloads and permanent SMTP failures (5xx) are not retried;
// everything else is returned as a plain error so asynq retries it. Errors
// never include the message body.
func (h *Handler) HandleTask(ctx context.Context, t *asynq.Task) error {
	var msg Message
	if err := json.Unmarshal(t.Payload(), &msg); err != nil {
		return fmt.Errorf("mailer: decode payload: %v: %w", err, asynq.SkipRetry)
	}
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("mailer: invalid message: %v: %w", err, asynq.SkipRetry)
	}

	if err := h.sender.Send(ctx, msg); err != nil {
		if isPermanentSMTPError(err) || errors.Is(err, ErrInsecureAuth) {
			return fmt.Errorf("mailer: permanent SMTP failure: %v: %w", err, asynq.SkipRetry)
		}
		return fmt.Errorf("mailer: send: %w", err)
	}
	return nil
}

// isPermanentSMTPError reports whether err carries an SMTP 5xx reply.
func isPermanentSMTPError(err error) bool {
	var tpErr *textproto.Error
	return errors.As(err, &tpErr) && tpErr.Code >= 500 && tpErr.Code < 600
}
