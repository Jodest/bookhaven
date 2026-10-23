// Package errtrack reports unhandled production errors to Sentry
// (docs/features/error-tracking.md). It is the only package that imports
// sentry-go.
//
// Reporting is opt-in: with an empty DSN, Init returns a no-op Reporter and no
// client is created. Events never carry the HTTP request, the user, breadcrumbs
// or task payloads, and strings are scrubbed of emails and tokens (see scrub).
package errtrack

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Options configures Init.
type Options struct {
	DSN         string
	Environment string
	Release     string
	// Component is the "component" tag: api / worker.
	Component string

	// transport overrides the HTTP transport (tests only).
	transport sentry.Transport
}

// HTTPFailure describes a request that ended with status >= 500.
type HTTPFailure struct {
	Method string
	// Route is the route template (c.Path()), never the raw URI.
	Route  string
	Status int
	// Err is the error the handler returned, if any.
	Err error
}

// TaskFailure describes a background task that failed for good. It carries no
// payload on purpose (emails, user content).
type TaskFailure struct {
	Type     string
	Queue    string
	Retried  int
	MaxRetry int
	Err      error
}

// Reporter sends errors to the tracker. It is the test seam of the API
// middleware and the worker error handler.
type Reporter interface {
	CaptureHTTP(ctx context.Context, f HTTPFailure)
	CapturePanic(ctx context.Context, recovered any, method, route string)
	CaptureTaskFailure(ctx context.Context, f TaskFailure)
}

// Enabled reports whether r actually sends events.
func Enabled(r Reporter) bool {
	if r == nil {
		return false
	}
	_, off := r.(noopReporter)
	return !off
}

type noopReporter struct{}

func (noopReporter) CaptureHTTP(context.Context, HTTPFailure)          {}
func (noopReporter) CapturePanic(context.Context, any, string, string) {}
func (noopReporter) CaptureTaskFailure(context.Context, TaskFailure)   {}

func noopFlush(time.Duration) {}

// Init creates the Reporter and the flush function for one process. It never
// fails: an empty DSN logs "error tracking disabled", an invalid one logs a
// warning (without the DSN value); both return a no-op Reporter.
func Init(opts Options, logger *zap.Logger) (Reporter, func(time.Duration)) {
	if logger == nil {
		logger = zap.NewNop()
	}
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		logger.Info("error tracking disabled")
		return noopReporter{}, noopFlush
	}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      opts.Environment,
		Release:          opts.Release,
		SampleRate:       1.0,
		AttachStacktrace: true,
		SendDefaultPII:   false,
		MaxBreadcrumbs:   -1,
		BeforeSend:       scrub,
		Transport:        opts.transport,
	})
	if err != nil {
		// err can echo the DSN; log only the fact.
		logger.Warn("error tracking disabled: invalid SENTRY_DSN")
		return noopReporter{}, noopFlush
	}
	hub := sentry.NewHub(client, sentry.NewScope())
	hub.ConfigureScope(func(s *sentry.Scope) {
		s.SetTag("component", opts.Component)
	})
	logger.Info("error tracking enabled",
		zap.String("environment", opts.Environment),
		zap.String("release", opts.Release),
		zap.String("component", opts.Component))
	return &reporter{hub: hub}, func(d time.Duration) { client.Flush(d) }
}

type reporter struct {
	hub *sentry.Hub
}

// clone returns a per-capture hub with its own copy of the scope, so
// concurrent requests never share tags (WithScope would mutate the shared
// scope stack).
func (r *reporter) clone() (*sentry.Hub, *sentry.Scope) {
	h := r.hub.Clone()
	return h, h.Scope()
}

func (r *reporter) CaptureHTTP(_ context.Context, f HTTPFailure) {
	h, s := r.clone()
	status := strconv.Itoa(f.Status)
	s.SetTag("method", f.Method)
	s.SetTag("route", f.Route)
	s.SetTag("status", status)
	s.SetFingerprint([]string{f.Method, f.Route, status})
	s.SetLevel(sentry.LevelError)
	msg := fmt.Sprintf("HTTP %d %s %s", f.Status, f.Method, f.Route)
	if f.Err != nil {
		h.CaptureException(f.Err)
		return
	}
	h.CaptureMessage(msg)
}

func (r *reporter) CapturePanic(_ context.Context, recovered any, method, route string) {
	h, s := r.clone()
	s.SetTag("method", method)
	s.SetTag("route", route)
	s.SetTag("status", "500")
	s.SetFingerprint([]string{method, route, "panic"})
	s.SetLevel(sentry.LevelFatal)
	h.Recover(recovered)
}

func (r *reporter) CaptureTaskFailure(_ context.Context, f TaskFailure) {
	h, s := r.clone()
	s.SetTag("task_type", f.Type)
	s.SetTag("queue", f.Queue)
	s.SetTag("retried", strconv.Itoa(f.Retried))
	s.SetTag("max_retry", strconv.Itoa(f.MaxRetry))
	s.SetLevel(sentry.LevelError)
	errType := "nil"
	if f.Err != nil {
		errType = fmt.Sprintf("%T", f.Err)
	}
	s.SetFingerprint([]string{f.Type, errType})
	msg := fmt.Sprintf("task %s failed: %v", f.Type, f.Err)
	// Message event: the attached exception would carry the raw error
	// type/stack of the worker library, the scrubbed text is what matters.
	h.CaptureMessage(msg)
}
