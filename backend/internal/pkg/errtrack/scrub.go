package errtrack

import (
	"regexp"

	"github.com/getsentry/sentry-go"
)

var (
	reEmail  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	reBearer = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=\-]+`)
	reJWT    = regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*`)
	reKV     = regexp.MustCompile(`(?i)\b(token|password|secret)=[^&\s"']+`)
)

// redact removes emails, bearer tokens, JWTs and token=/password=/secret=
// values from s (best effort).
func redact(s string) string {
	s = reEmail.ReplaceAllString(s, "[email]")
	s = reBearer.ReplaceAllString(s, "Bearer [redacted]")
	s = reJWT.ReplaceAllString(s, "[jwt]")
	s = reKV.ReplaceAllString(s, "${1}=[redacted]")
	return s
}

// scrub is the BeforeSend hook: it applies the redactions and strips request
// and user data. It never drops the event.
func scrub(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	event.Message = redact(event.Message)
	for i := range event.Exception {
		event.Exception[i].Value = redact(event.Exception[i].Value)
	}
	if event.Request != nil {
		event.Request.Cookies = ""
		event.Request.Headers = nil
		event.Request.Data = ""
		event.Request.QueryString = ""
		event.Request.Env = nil
	}
	event.User = sentry.User{}
	return event
}
