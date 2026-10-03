// Command mailtest is a dev-only CLI that enqueues one test email
// (email:send task) via mailer.Queue, for a manual end-to-end check of the
// mail pipeline against Mailpit. It never talks to SMTP itself - the
// worker delivers the task. Not included in the production image.
//
//	docker compose up -d
//	docker compose run --rm backend go run ./cmd/mailtest -to test@example.com
//
// then open the Mailpit UI at http://localhost:8025.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bookhaven/backend/internal/config"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/hibiken/asynq"
)

const (
	defaultSubject = "BookHaven: тестовое письмо"
	defaultText    = "Привет! Это тестовое письмо BookHaven.\n\nЕсли вы видите этот текст, отправка почты работает."
	defaultHTML    = "<p>Привет! Это <strong>тестовое письмо</strong> BookHaven.</p><p>Если вы видите этот текст, отправка почты работает.</p>"
)

// recordingEnqueuer wraps *asynq.Client to capture the TaskInfo of the
// enqueued task (mailer.Queue itself only returns an error).
type recordingEnqueuer struct {
	client *asynq.Client
	info   *asynq.TaskInfo
}

func (r *recordingEnqueuer) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := r.client.EnqueueContext(ctx, task, opts...)
	r.info = info
	return info, err
}

func main() {
	to := flag.String("to", "", "recipient address (required)")
	subject := flag.String("subject", defaultSubject, "email subject")
	text := flag.String("text", defaultText, "plain-text body")
	html := flag.String("html", defaultHTML, "HTML body (pass an empty string for text-only)")
	flag.Parse()

	if strings.TrimSpace(*to) == "" {
		fmt.Fprintln(os.Stderr, "mailtest: -to is required")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mailtest: load config:", err)
		os.Exit(1)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisURL})
	defer client.Close()

	rec := &recordingEnqueuer{client: client}
	msg := mailer.Message{To: *to, Subject: *subject, TextBody: *text, HTMLBody: *html}
	if err := mailer.NewQueue(rec).Enqueue(context.Background(), msg); err != nil {
		fmt.Fprintln(os.Stderr, "mailtest:", err)
		os.Exit(1)
	}

	fmt.Printf("enqueued task id=%s queue=%s\n", rec.info.ID, rec.info.Queue)
}
