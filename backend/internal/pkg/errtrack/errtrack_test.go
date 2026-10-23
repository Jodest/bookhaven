package errtrack

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// recordingTransport keeps events in memory; nothing touches the network.
type recordingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *recordingTransport) Flush(time.Duration) bool              { return true }
func (t *recordingTransport) FlushWithContext(context.Context) bool { return true }
func (t *recordingTransport) Configure(sentry.ClientOptions)        {}
func (t *recordingTransport) Close()                                {}
func (t *recordingTransport) SendEvent(e *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, e)
}

const fakeDSN = "https://key@o0.ingest.invalid/0"

func newTestReporter(t *testing.T) (Reporter, *recordingTransport, *observer.ObservedLogs) {
	t.Helper()
	tr := &recordingTransport{}
	core, logs := observer.New(zap.InfoLevel)
	r, _ := Init(Options{DSN: fakeDSN, Environment: "test", Release: "r1", Component: "api", transport: tr}, zap.New(core))
	return r, tr, logs
}

func TestInit_EmptyDSNIsNoop(t *testing.T) {
	for _, dsn := range []string{"", "   "} {
		core, logs := observer.New(zap.InfoLevel)
		r, flush := Init(Options{DSN: dsn, Component: "api"}, zap.New(core))
		if Enabled(r) {
			t.Errorf("dsn %q: reporter must be a no-op", dsn)
		}
		r.CaptureHTTP(context.Background(), HTTPFailure{Method: "GET", Route: "/x", Status: 500})
		r.CapturePanic(context.Background(), "boom", "GET", "/x")
		r.CaptureTaskFailure(context.Background(), TaskFailure{Type: "t"})
		flush(time.Millisecond)
		if logs.FilterMessage("error tracking disabled").Len() != 1 {
			t.Errorf("dsn %q: want one 'error tracking disabled' line, got %v", dsn, logs.All())
		}
	}
}

func TestInit_InvalidDSNWarnsAndIsNoop(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	r, flush := Init(Options{DSN: "not a dsn://secret-key"}, zap.New(core))
	if Enabled(r) {
		t.Fatal("invalid DSN must give a no-op reporter")
	}
	flush(time.Millisecond)
	warns := logs.FilterLevelExact(zap.WarnLevel).All()
	if len(warns) != 1 {
		t.Fatalf("want one warning, got %v", logs.All())
	}
	if strings.Contains(warns[0].Message, "secret-key") {
		t.Error("warning must not contain the DSN")
	}
}

func TestInit_EnabledLogsWithoutDSN(t *testing.T) {
	_, _, logs := newTestReporter(t)
	if logs.FilterMessage("error tracking enabled").Len() != 1 {
		t.Fatalf("want 'error tracking enabled' line, got %v", logs.All())
	}
	for _, e := range logs.All() {
		if strings.Contains(e.Message, "ingest.invalid") {
			t.Error("DSN leaked into logs")
		}
	}
}

func TestCaptureHTTP_Message(t *testing.T) {
	r, tr, _ := newTestReporter(t)
	r.CaptureHTTP(context.Background(), HTTPFailure{Method: "GET", Route: "/api/v1/shelves/:id", Status: 500})
	if len(tr.events) != 1 {
		t.Fatalf("want 1 event, got %d", len(tr.events))
	}
	e := tr.events[0]
	if e.Message != "HTTP 500 GET /api/v1/shelves/:id" {
		t.Errorf("message = %q", e.Message)
	}
	want := map[string]string{"method": "GET", "route": "/api/v1/shelves/:id", "status": "500", "component": "api"}
	for k, v := range want {
		if e.Tags[k] != v {
			t.Errorf("tag %s = %q, want %q", k, e.Tags[k], v)
		}
	}
	if strings.Join(e.Fingerprint, "|") != "GET|/api/v1/shelves/:id|500" {
		t.Errorf("fingerprint = %v", e.Fingerprint)
	}
	if e.Request != nil || !e.User.IsEmpty() {
		t.Errorf("request/user must be empty: %+v %+v", e.Request, e.User)
	}
	if e.Environment != "test" || e.Release != "r1" {
		t.Errorf("env/release = %q/%q", e.Environment, e.Release)
	}
	if len(e.Breadcrumbs) != 0 {
		t.Errorf("breadcrumbs = %v", e.Breadcrumbs)
	}
}

func TestCaptureHTTP_ErrorBecomesException(t *testing.T) {
	r, tr, _ := newTestReporter(t)
	r.CaptureHTTP(context.Background(), HTTPFailure{Method: "POST", Route: "/x", Status: 500, Err: errors.New("db down for a@b.com")})
	if len(tr.events) != 1 || len(tr.events[0].Exception) == 0 {
		t.Fatalf("want 1 event with exception, got %+v", tr.events)
	}
	if v := tr.events[0].Exception[0].Value; v != "db down for [email]" {
		t.Errorf("exception value = %q", v)
	}
}

func TestCapturePanic(t *testing.T) {
	r, tr, _ := newTestReporter(t)
	r.CapturePanic(context.Background(), "kaboom", "GET", "/p/:id")
	if len(tr.events) != 1 {
		t.Fatalf("want 1 event, got %d", len(tr.events))
	}
	if tr.events[0].Tags["route"] != "/p/:id" || tr.events[0].Tags["component"] != "api" {
		t.Errorf("tags = %v", tr.events[0].Tags)
	}
}

func TestCaptureTaskFailure(t *testing.T) {
	r, tr, _ := newTestReporter(t)
	r.CaptureTaskFailure(context.Background(), TaskFailure{
		Type: "email:send", Queue: "mail", Retried: 8, MaxRetry: 8,
		Err: errors.New("smtp refused rcpt user@example.com"),
	})
	if len(tr.events) != 1 {
		t.Fatalf("want 1 event, got %d", len(tr.events))
	}
	e := tr.events[0]
	if e.Message != "task email:send failed: smtp refused rcpt [email]" {
		t.Errorf("message = %q", e.Message)
	}
	want := map[string]string{"task_type": "email:send", "queue": "mail", "retried": "8", "max_retry": "8"}
	for k, v := range want {
		if e.Tags[k] != v {
			t.Errorf("tag %s = %q, want %q", k, e.Tags[k], v)
		}
	}
	// sentry-go v0.50 removed Event.Extra, so request and contexts are the
	// remaining channels a payload could leak through.
	if e.Request != nil || len(e.Contexts["payload"]) != 0 {
		t.Error("no request/payload allowed")
	}
}

func TestScrub(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig_nature-1"
	ev := &sentry.Event{
		Message: "fail for a.b+c@mail.example.org with Bearer abc.def-123 and " + jwt + " ?token=xyz&x=1 password=hunter2",
		Exception: []sentry.Exception{
			{Value: "user bob@example.com secret=s3 " + jwt},
		},
		Request: &sentry.Request{
			Cookies: "sid=1", Headers: map[string]string{"Authorization": "x"},
			Data: "body", QueryString: "q=1", Env: map[string]string{"A": "b"}, URL: "https://h/p",
		},
		User: sentry.User{ID: "1", Email: "a@b.com", IPAddress: "1.2.3.4"},
	}
	got := scrub(ev, nil)
	if got == nil {
		t.Fatal("scrub must not drop the event")
	}
	for _, bad := range []string{"@", "abc.def-123", "eyJ", "xyz", "hunter2", "s3"} {
		if strings.Contains(got.Message, bad) || strings.Contains(got.Exception[0].Value, bad) {
			t.Errorf("%q survived: %q / %q", bad, got.Message, got.Exception[0].Value)
		}
	}
	for _, want := range []string{"[email]", "Bearer [redacted]", "[jwt]", "token=[redacted]", "password=[redacted]"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message missing %q: %q", want, got.Message)
		}
	}
	r := got.Request
	if r.Cookies != "" || r.Headers != nil || r.Data != "" || r.QueryString != "" || r.Env != nil {
		t.Errorf("request not stripped: %+v", r)
	}
	if !got.User.IsEmpty() {
		t.Errorf("user not stripped: %+v", got.User)
	}
}
