// Package coverfetch downloads book cover images safely (#489): only
// allowlisted hosts over http/https on ports 80/443, public addresses only
// (checked at dial time, also for redirect hops), bounded size and time, and
// only JPEG/PNG/WebP by magic bytes.
package coverfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Error codes stored in books.cover_error.
const (
	CodeHostNotAllowed   = "host_not_allowed"
	CodeBlockedAddress   = "blocked_address"
	CodeTooManyRedirects = "too_many_redirects"
	CodeTooLarge         = "too_large"
	CodeNotImage         = "not_image"
	CodeHTTPError        = "http_error"
	CodeTimeout          = "timeout"
	CodeNetwork          = "network"
)

const (
	// MaxBytes is the largest accepted image.
	MaxBytes = 5 << 20
	// MaxRedirects is the number of redirects followed.
	MaxRedirects = 5
	// Timeout bounds the whole download.
	Timeout = 30 * time.Second

	openLibraryHost = "covers.openlibrary.org"
)

// DefaultAllowedHosts is the default allowlist.
const DefaultAllowedHosts = "covers.openlibrary.org,archive.org,*.archive.org,upload.wikimedia.org,image.tmdb.org"

// DefaultLinkOnlyHosts is the default list of hosts whose covers are shown by
// link and never downloaded (see covers/service WithLinkOnlyHosts). Matched
// with MatchHost.
const DefaultLinkOnlyHosts = "fantlab.ru,*.fantlab.ru"

// Error is a classified download failure.
type Error struct {
	Code string
	// Transient errors are retried with backoff; the rest are permanent.
	Transient bool
	Err       error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("cover download: %s: %v", e.Code, e.Err)
	}
	return "cover download: " + e.Code
}

func (e *Error) Unwrap() error { return e.Err }

func newErr(code string, transient bool, err error) *Error {
	return &Error{Code: code, Transient: transient, Err: err}
}

// Result is a downloaded image, or Absent when the source says it has none.
type Result struct {
	Data   []byte
	Ext    string // "jpg", "png" or "webp"
	Absent bool
}

// Fetcher downloads covers.
type Fetcher struct {
	client  *http.Client
	allowed []string
	timeout time.Duration
}

// New returns a Fetcher with the SSRF-guarded transport.
func New(allowedHosts []string) *Fetcher {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: guardControl}
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     true,
	}
	return NewWithTransport(allowedHosts, tr)
}

// NewWithTransport builds a Fetcher on a custom RoundTripper (tests inject a
// fake one; the dial guard then does not apply).
func NewWithTransport(allowedHosts []string, rt http.RoundTripper) *Fetcher {
	f := &Fetcher{allowed: allowedHosts, timeout: Timeout}
	f.client = &http.Client{
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxRedirects {
				return newErr(CodeTooManyRedirects, false, nil)
			}
			if e := f.checkURL(req.URL); e != nil {
				return e
			}
			return nil
		},
	}
	return f
}

// checkURL enforces scheme, port and host allowlist.
func (f *Fetcher) checkURL(u *url.URL) *Error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return newErr(CodeHostNotAllowed, false, errors.New("scheme not allowed"))
	}
	switch u.Port() {
	case "", "80", "443":
	default:
		return newErr(CodeHostNotAllowed, false, errors.New("port not allowed"))
	}
	if !MatchHost(f.allowed, u.Hostname()) {
		return newErr(CodeHostNotAllowed, false, errors.New("host not on the allowlist"))
	}
	return nil
}

// Fetch downloads rawURL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, newErr(CodeHostNotAllowed, false, err)
	}
	if e := f.checkURL(u); e != nil {
		return nil, e
	}
	isOL := strings.EqualFold(u.Hostname(), openLibraryHost)
	if isOL {
		q := u.Query()
		if !q.Has("default") {
			q.Set("default", "false")
			u.RawQuery = q.Encode()
		}
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, newErr(CodeHostNotAllowed, false, err)
	}
	req.Header.Set("User-Agent", "BookHaven-CoverMirror/1.0")
	req.Header.Set("Accept", "image/jpeg,image/png,image/webp,*/*;q=0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, classify(ctx, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound && isOL:
		return &Result{Absent: true}, nil
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, newErr(CodeHTTPError, true, fmt.Errorf("status %d", resp.StatusCode))
	default:
		return nil, newErr(CodeHTTPError, false, fmt.Errorf("status %d", resp.StatusCode))
	}
	if resp.ContentLength > MaxBytes {
		return nil, newErr(CodeTooLarge, false, nil)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, classify(ctx, err)
	}
	if len(data) > MaxBytes {
		return nil, newErr(CodeTooLarge, false, nil)
	}
	ext := SniffExt(data)
	if ext == "" {
		return nil, newErr(CodeNotImage, false, nil)
	}
	return &Result{Data: data, Ext: ext}, nil
}

// SniffExt detects JPEG, PNG and WebP by magic bytes; "" for anything else.
func SniffExt(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpg"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp"
	}
	return ""
}

func classify(ctx context.Context, err error) *Error {
	var fe *Error
	if errors.As(err, &fe) {
		return fe
	}
	if errors.Is(err, ErrBlockedAddress) {
		return newErr(CodeBlockedAddress, false, err)
	}
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return newErr(CodeTimeout, true, err)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return newErr(CodeTimeout, true, err)
	}
	return newErr(CodeNetwork, true, err)
}
