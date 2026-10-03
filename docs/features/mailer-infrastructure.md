# Mailer Infrastructure (SMTP sender + async email queue)

Status: implemented

GitHub issue: #157. First link of the "email verification at registration" chain: #158 (verification backend), #159 (verification frontend), #160 (restrictions for unverified users), #161 (password reset by email). #121 (email notifications) will also build on it. This contract only covers the plumbing; no concrete email is sent by any user flow in this increment.

## Goal

Give the backend one reusable way to send transactional email: a service in the `api` process calls `Enqueue(ctx, mailer.Message)`, an asynq task `email:send` carries the already-rendered message to the `worker`, and the worker delivers it via SMTP (or logs it when SMTP is not configured) with automatic retries. Developers see every email locally in Mailpit.

## Scope boundaries

**In scope:**
1. New package `backend/internal/pkg/mailer`: `Message`, `Sender` interface, `SMTPSender`, `LogSender`, sender selection from config, `email:send` task (type/queue/payload/constructor/handler), and an `Enqueuer` for callers.
2. Config: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `MAIL_FROM`.
3. `cmd/worker`: build the sender, register the `email:send` handler on a dedicated asynq server for the `email` queue.
4. `cmd/mailtest`: tiny dev CLI that enqueues one test email (manual end-to-end check against Mailpit).
5. Mailpit in `docker-compose.yml`; worker wired to it by default.
6. `docker-compose.prod.yml` + `.env.production.example` env passthrough for the worker; `docs/DEPLOYMENT.md` SMTP/SPF/DKIM/DMARC section.
7. Template convention for future features (documented here, not implemented).

**Out of scope:**
- Any concrete email (verification, password reset, notifications), any template file, any change to `auth`/`notifications` modules.
- Wiring the enqueuer into `cmd/api` (no consumer yet - a declared-but-unused client would be dead code; #158 does this, see "How later features use this").
- DB schema changes, HTTP endpoints, frontend changes.
- Bounce/complaint webhooks, unsubscribe handling, rate limiting per recipient, email log table.
- Changing the existing harvest queue/server configuration (see Risks for a latent issue found there).

## User Stories

- As a backend developer building #158/#161/#121, I enqueue an email from a service with one call and do not touch asynq, SMTP or retries myself.
- As a developer, I run `docker compose up` and see every email the system sends in the Mailpit web UI at `http://localhost:8025`.
- As a developer running the backend natively without SMTP, nothing crashes: emails are written to the worker log instead.
- As an operator, I configure an SMTP provider through five env vars and the DEPLOYMENT guide tells me which DNS records to add so mail is not marked as spam.
- As the system, a transient SMTP failure is retried with backoff; a permanent rejection is not retried forever.

## Acceptance Criteria

- [ ] `backend/internal/pkg/mailer` exists with the API described in the API section; no new third-party dependency (stdlib `net/smtp`, `crypto/tls`, `mime`, `mime/multipart`, `mime/quotedprintable`, `net/mail`).
- [ ] `config.Config` gains `SMTPHost`, `SMTPPort` (default `587`), `SMTPUser`, `SMTPPassword`, `MailFrom` (default `BookHaven <noreply@bookhaven.local>`), loaded via the existing `getEnv`. `config.Load` returns an error if `SMTP_HOST` is set and `MAIL_FROM` is not a valid address (`net/mail.ParseAddress`), or if `SMTP_PORT` is not a number.
- [ ] `mailer.NewSender(cfg, logger)` returns `SMTPSender` when `SMTPHost != ""`, otherwise `LogSender`. The worker logs which sender is active at startup (host:port and from-address for SMTP; never the password). In `APP_ENV=production` with empty `SMTP_HOST` it logs a `Warn` ("SMTP not configured, emails will only be logged") but does not exit.
- [ ] `SMTPSender.Send` delivers a correctly-formed MIME message: `From`, `To`, `Subject` (RFC 2047 encoded via `mime.QEncoding` - subjects will be Cyrillic), `Date`, `Message-ID`, `MIME-Version: 1.0`; `text/plain` only if `HTMLBody` is empty, otherwise `multipart/alternative` with the text part first and HTML part second; both parts `charset=UTF-8`, `Content-Transfer-Encoding: quoted-printable`.
- [ ] TLS handling (no extra env var): port `465` -> implicit TLS (`tls.Dial` + `smtp.NewClient`); any other port -> plain connect, then `STARTTLS` if the server advertises it. If `SMTP_USER` is set, `smtp.PlainAuth` is used (stdlib already refuses PLAIN auth over an unencrypted non-localhost connection - that is the desired behavior: credentials are never sent in clear). If `SMTP_USER` is empty, no AUTH (Mailpit). `tls.Config.ServerName = SMTPHost`; no `InsecureSkipVerify`. Dial/command deadline bounded (e.g. 30s, and respecting `ctx`).
- [ ] `LogSender.Send` logs `to` and `subject` at Info. It also logs the text body only when `APP_ENV != production` (dev convenience: verification links visible in logs; never in prod logs because bodies will contain tokens). Always returns `nil`.
- [ ] `mailer.TaskTypeSendEmail = "email:send"`, `mailer.QueueName = "email"`, `mailer.NewSendEmailTask(msg)` builds the task with `asynq.Queue(QueueName)`, `asynq.MaxRetry(8)`, `asynq.Timeout(1*time.Minute)` and no `Retention` (completed tasks are deleted immediately - payloads will contain tokens).
- [ ] `mailer.Message.Validate()` rejects: empty or unparseable `To` (`net/mail.ParseAddress`), empty `Subject`, empty `TextBody`, any CR/LF in `To` or `Subject` (header injection). `NewSendEmailTask` and `Handler.HandleTask` both call it.
- [ ] `mailer.Handler.HandleTask`: malformed JSON or invalid message -> `asynq.SkipRetry`; SMTP permanent failure (`*textproto.Error` with code 5xx) -> `asynq.SkipRetry`; anything else (network, timeouts, 4xx) -> plain error (asynq retries). Errors never include the message body.
- [ ] `mailer.Queue` (implements `mailer.Enqueuer`) wraps a `TaskEnqueuer` (`Enqueue(*asynq.Task, ...asynq.Option)`), satisfied by the existing `*asynq.Client`, and `Enqueue(ctx, msg)` validates then enqueues via `EnqueueContext`.
- [ ] `cmd/worker/main.go` runs a **second** `asynq.Server` for the email queue (`Queues: {"email": 1}`, `Concurrency: 4`) with its own `ServeMux` registering `mailer.TaskTypeSendEmail`; both servers start, and both are shut down on SIGINT/SIGTERM. The existing harvest server and its config are untouched.
- [ ] `backend/cmd/mailtest/main.go`: flags `-to` (required), `-subject`, `-text`, `-html` (optional, sensible defaults incl. Cyrillic subject); enqueues one task via `mailer.Queue` using `cfg.RedisURL`; prints task ID; no SMTP access itself. Dev tool only - not added to the production Dockerfile stage.
- [ ] `docker-compose.yml`: `mailpit` service (`axllent/mailpit`, container `bookhaven-mailpit`, ports `${MAILPIT_SMTP_PORT:-1025}:1025` and `${MAILPIT_UI_PORT:-8025}:8025`); `worker` gets `SMTP_HOST: mailpit`, `SMTP_PORT: 1025`, `MAIL_FROM: BookHaven <noreply@bookhaven.local>` and `depends_on: mailpit`. `.env.example` documents `MAILPIT_SMTP_PORT`/`MAILPIT_UI_PORT`.
- [ ] `docker-compose.prod.yml`: worker `environment` gains `SMTP_HOST: ${SMTP_HOST:-}`, `SMTP_PORT: ${SMTP_PORT:-587}`, `SMTP_USER: ${SMTP_USER:-}`, `SMTP_PASSWORD: ${SMTP_PASSWORD:-}`, `MAIL_FROM: ${MAIL_FROM:-}`. `backend` (api) does not get them. `.env.production.example` gains the same keys with comments.
- [ ] Manual check: `docker compose up`, `docker compose run --rm backend go run ./cmd/mailtest -to test@example.com` -> the email (Cyrillic subject renders correctly, HTML and text parts present) appears in Mailpit UI.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` clean in `backend/` (run via Docker per dev environment).
- [ ] Docs updated (see Documentation).

## Business Rules

- Delivery is always asynchronous: no request handler ever talks to SMTP directly. `Enqueue` failing (Redis down) is the caller's problem to handle - later features decide whether that fails the request (e.g. #161 password reset should return an error) or is only logged (e.g. #158 Register succeeds, user can re-request verification).
- At-least-once delivery: asynq may retry after a send that actually succeeded (e.g. connection dropped after `DATA` accepted). A duplicate email is acceptable; a lost one is worse. Consumers must design emails to be harmless when duplicated (links with tokens stay valid until used/expired).
- Retry policy: `MaxRetry(8)` with asynq default backoff covers roughly a few hours of provider outage; after that the task is archived (visible via asynq Inspector) and not retried further.
- Permanent SMTP rejections (5xx) are not retried.
- Rendering happens in the caller (api process) at enqueue time; the worker sends exactly what is in the payload and has no knowledge of email types.
- Secrets: `SMTP_PASSWORD` is never logged; email bodies are never logged in production; completed tasks are not retained in Redis.
- One recipient per message (`To` is a single address). Bulk sends (#121 digests) enqueue one task per recipient.

## API

No HTTP API. Go API of `backend/internal/pkg/mailer` (file split suggestion: `mailer.go`, `smtp.go`, `log.go`, `task.go`, `queue.go`):

```go
type Message struct {
    To       string `json:"to"`
    Subject  string `json:"subject"`
    TextBody string `json:"text_body"`           // required
    HTMLBody string `json:"html_body,omitempty"` // optional
}
func (m Message) Validate() error

type Sender interface {
    Send(ctx context.Context, msg Message) error
}
func NewSender(cfg *config.Config, logger *zap.Logger) Sender // SMTPSender or LogSender
func NewSMTPSender(opts SMTPOptions) *SMTPSender               // Host, Port, User, Password, From
func NewLogSender(logger *zap.Logger, logBodies bool) *LogSender

const TaskTypeSendEmail = "email:send"
const QueueName = "email"
func NewSendEmailTask(msg Message) (*asynq.Task, error)

type Handler struct{ /* sender Sender */ }
func NewHandler(sender Sender) *Handler
func (h *Handler) HandleTask(ctx context.Context, t *asynq.Task) error

// For callers in the api process.
type Enqueuer interface {
    Enqueue(ctx context.Context, msg Message) error
}
type TaskEnqueuer interface {
    EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
type Queue struct{ /* client TaskEnqueuer */ }
func NewQueue(client TaskEnqueuer) *Queue // implements Enqueuer
```

Message building (MIME encoding) should be a separate pure function (e.g. `buildMessage(from string, msg Message, now time.Time, messageID string) []byte`) so it is unit-testable without a network.

### How later features use this (contract for #158/#161/#121)

> Implemented by #158: `mailer.Render` in `backend/internal/pkg/mailer/render.go` with embedded templates in `templates/` (first template: `verify_email`), `WEB_BASE_URL` config, and the `cmd/api` wiring (`mailer.NewQueue(taskClient)`). See [email-verification-backend.md](email-verification-backend.md).

- `cmd/api/main.go`: reuse the existing `asynq.Client` (currently named `harvestClient`; rename to e.g. `taskClient` when the second consumer appears) -> `mailer.NewQueue(taskClient)` -> inject into the consuming service.
- The consuming service depends on `mailer.Enqueuer` (or its own one-method interface with the same shape); tests use a hand-rolled fake recording messages (no mocking library, per repo convention).
- Templates: the first feature that needs one (#158) adds `backend/internal/pkg/mailer/templates/` embedded with `//go:embed`, a shared `layout.html.tmpl`, and per-email pairs `<name>.html.tmpl` + `<name>.txt.tmpl`, rendered by a helper such as `mailer.Render(name string, data any) (Message-without-To, error)` using `html/template` for HTML (auto-escaping user-provided values like usernames) and `text/template` for the text part. Subject lives in the template set (e.g. a `{{define "subject"}}` block in the txt template). Email copy is Russian (product is Russian-only). Absolute links are built from a new `WEB_BASE_URL`-style config value added by #158, not hardcoded.

## Database

None.

## Frontend

None.

## Existing Patterns

- Task type / queue constants, payload struct, `New...Task` constructor fixing retry/timeout options, `SkipRetry` for bad payloads: `backend/internal/modules/harvest/service/handler.go` (`TaskTypeHarvestWikidataQuery`, `QueueName`, `NewHarvestTask`, `HandleTask`).
- Small enqueuer interface over `*asynq.Client` + hand-rolled fake in tests: `backend/internal/modules/harvest/transport/http/handler.go` (`TaskEnqueuer`) and its `handler_test.go` (`fakeEnqueuer`).
- Worker server setup, zap->asynq logger adapter, graceful shutdown: `backend/cmd/worker/main.go` (reuse `zapAsynqLogger` for the second server).
- Enqueue-only CLI: `backend/cmd/harvest/main.go` (model for `cmd/mailtest`).
- Config loading with `getEnv` defaults and production guard: `backend/internal/config/config.go` (`JWT_SECRET` check).
- Infrastructure package placement: `backend/internal/pkg/bookdata` (mailer is shared infra, not a domain module, so it goes under `internal/pkg`, not `internal/modules`).

## Tests

All without real SMTP or Redis.
- `Message.Validate`: valid; empty/invalid `To`; empty subject; empty text body; CR/LF in `To`/`Subject`.
- `NewSendEmailTask`: type, queue, `MaxRetry`, `Timeout` (inspect via `asynq.Task` / options as harvest tests do), payload round-trip.
- `Handler.HandleTask` with a fake `Sender`: success passes the decoded message; malformed JSON -> `errors.Is(err, asynq.SkipRetry)`; invalid message -> SkipRetry, sender not called; sender returns `&textproto.Error{Code: 550}` -> SkipRetry; `Code: 451` and a generic network error -> retryable (not SkipRetry).
- `Queue.Enqueue` with fake `TaskEnqueuer`: valid message enqueued with correct type/payload; invalid message -> error, nothing enqueued; enqueuer error propagated.
- `NewSender`: empty host -> `*LogSender`; host set -> `*SMTPSender`.
- `buildMessage`: headers present; Cyrillic subject is Q/B-encoded and decodes back via `mime.WordDecoder`; text-only -> single `text/plain` part; text+HTML -> `multipart/alternative` with both parts, parseable by `net/mail` + `mime/multipart`.
- `LogSender`: body logged only when `logBodies` is true (zap `observer` core).
- `config.Load`: `SMTP_HOST` set + invalid `MAIL_FROM` -> error; non-numeric `SMTP_PORT` -> error; defaults when unset.
- Optional: `SMTPSender` against an in-process fake SMTP server on `127.0.0.1` (net.Listener speaking minimal SMTP) - only if it stays small; the manual Mailpit check covers the real path.

## Documentation

- `docs/DEPLOYMENT.md`: new section "Email (SMTP)" covering:
  - Env vars and behavior when `SMTP_HOST` is empty (log-only; emails effectively disabled).
  - Choosing a provider: a transactional SMTP service with a Russian-reachable endpoint (e.g. Unisender Go, SendPulse, Yandex 360 for small volume; Postmark/Mailgun/Amazon SES if reachable/payable). Use port 587 (STARTTLS) or 465 (TLS); many VPS providers block outbound port 25.
  - DNS: SPF TXT on the sender domain including the provider (`v=spf1 include:<provider> ~all`, merged with any existing SPF record - only one SPF record allowed), DKIM record(s) issued by the provider, DMARC TXT at `_dmarc.<domain>` starting with `p=none; rua=mailto:...` and tightening to `quarantine` after monitoring. `MAIL_FROM` domain must be the domain these records are on.
  - Verification: send via `cmd/mailtest` equivalent or the first real flow, check headers (`spf=pass`, `dkim=pass`, `dmarc=pass`) in a Gmail/Yandex inbox.
  - Add the SMTP vars to "One-time server setup" step 5 and `SMTP_PASSWORD` to the Security checklist.
- `docs/ROADMAP.md`: under "Up Next" add the "Email verification at registration" chain in order - #157 mailer infrastructure (link this contract), #158, #159, #160, #161; note #121 depends on #157. On completion, move #157 to "Completed" with a link to this contract.
- Package doc comment in `mailer` explaining the async flow, at-least-once semantics, and the template convention above.
- `README`/dev docs (wherever local services are listed): mention Mailpit UI at `http://localhost:8025`.

## Implementation Notes

- Files: `backend/internal/pkg/mailer/{mailer.go,smtp.go,log.go,task.go,queue.go}` + tests; `backend/cmd/mailtest/main.go`; `backend/cmd/worker/main.go` (second `asynq.Server`); `backend/internal/config/config.go` (`SMTPPort` is an `int`; `SMTP_PORT` must be 1-65535).
- `NewSender` itself logs the active sender (SMTP: `addr`, `from`, `auth` bool; never the password) and the production `Warn`, so the worker only calls it.
- `SMTPSender`: whole conversation bounded by the earlier of 30s and the `ctx` deadline (connection deadline + close on `ctx` cancel); `tls.Config{ServerName: host, MinVersion: TLS 1.2}`; envelope uses the bare addresses parsed from `MAIL_FROM` / `To`. A failing `QUIT` after the message was accepted is ignored (no retry -> no duplicate). `Message-ID` is `<random-hex@MAIL_FROM-domain>`.
- Task options live in an unexported `sendEmailTaskOptions()` so tests can assert queue/MaxRetry/Timeout/no-Retention via `asynq.Option.Type()/Value()` (an `*asynq.Task` does not expose its options).
- `cmd/mailtest` wraps `*asynq.Client` in a small recording `TaskEnqueuer` to print the task ID, since `mailer.Queue.Enqueue` returns only an error. Not built in the production Dockerfile stage.
- `docker-compose.yml`: worker `depends_on: mailpit` uses `condition: service_started` (the other dependencies use the long syntax).
- Tests include a minimal in-process fake SMTP server (`smtp_test.go`) covering successful delivery and a 550 `RCPT` rejection surfacing as `*textproto.Error`.
- Manual Mailpit check done: `go run ./cmd/mailtest -to test@example.com` -> message in Mailpit with the Cyrillic subject decoded correctly and both text and HTML parts.

## Risks

- **Latent harvest concurrency issue (pre-existing, do not fix here):** `cmd/worker` comments claim the harvest queue runs with "concurrency 1", but asynq `Queues` map values are priorities and `Config.Concurrency` is unset (defaults to NumCPU), so multiple harvest tasks can run in parallel today. This is why email gets its own `asynq.Server` instead of sharing one: setting `Concurrency: 1` on a shared server would block email behind a 30-minute harvest. Recommend filing a separate issue to set `Concurrency: 1` on the harvest server.
- **Hand-rolled MIME:** stdlib `net/smtp` has no message builder; incorrect encoding of Cyrillic subjects/bodies is the most likely bug. Mitigated by `buildMessage` unit tests and the manual Mailpit check. If it grows unwieldy, `github.com/wneessen/go-mail` is the acceptable fallback dependency - flag it back rather than switching silently.
- **`net/smtp` is frozen** (no new features) but is maintained for security fixes and is sufficient for authenticated submission over TLS/STARTTLS.
- **Sensitive payloads in Redis:** future payloads carry tokens. Completed tasks are not retained, but archived (failed) tasks keep their payload until asynq's archive trimming. Acceptable given tokens expire; noted for #158/#161.
- **Silent no-op in production:** if `SMTP_HOST` is forgotten in prod, emails are only logged. Startup `Warn` mitigates; #158 may decide to make `SMTP_HOST` mandatory in production once verification is required.
- **Duplicate sends on retry** (at-least-once) - accepted, see Business Rules.
- **Deliverability** depends on DNS (SPF/DKIM/DMARC) set up outside the codebase; without it, mail lands in spam. Documented, not enforceable in code.
