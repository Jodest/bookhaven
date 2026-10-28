package http

import (
	nethttp "net/http"
	"testing"
	"time"
)

func TestApplyServerTimeouts(t *testing.T) {
	s := &nethttp.Server{}
	ApplyServerTimeouts(s, ServerTimeouts{ReadHeader: time.Second, Read: 2 * time.Second, Write: 3 * time.Second, Idle: 4 * time.Second})
	if s.ReadHeaderTimeout != time.Second || s.ReadTimeout != 2*time.Second || s.WriteTimeout != 3*time.Second || s.IdleTimeout != 4*time.Second {
		t.Errorf("timeouts not applied: %+v", s)
	}
}

func TestBodyLimitOverrides_Import(t *testing.T) {
	if got := BodyLimitOverrides()["POST /api/v1/admin/books/import"]; got != 2<<20 {
		t.Errorf("import override = %d", got)
	}
}

func TestBodyLimitOverrides_PosterUpload(t *testing.T) {
	if got := BodyLimitOverrides()["PUT /api/v1/admin/screen-works/:id/poster"]; got != 6<<20 {
		t.Errorf("poster override = %d, want %d", got, 6<<20)
	}
}
