package bookdata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// roundTripFunc adapts a function to http.RoundTripper, for fake HTTP
// transports in tests. No real network calls are made anywhere in this
// package's tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestResponse(status int, body string, headers map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp
}

// fakeLimiter counts Wait calls and never actually blocks, so tests run
// instantly and deterministically.
type fakeLimiter struct {
	waitCalls int
}

func (f *fakeLimiter) Wait(ctx context.Context) error {
	f.waitCalls++
	return nil
}

// newTestConfig builds a config wired to responder via a fake RoundTripper,
// a fake (instant) limiter, and a recording (non-sleeping) sleep func.
func newTestConfig(t *testing.T, responder roundTripFunc, extraOpts ...Option) (*config, *fakeLimiter, *[]time.Duration) {
	t.Helper()
	fl := &fakeLimiter{}
	var sleeps []time.Duration
	sleepFn := func(d time.Duration) { sleeps = append(sleeps, d) }

	opts := append([]Option{
		WithHTTPClient(&http.Client{Transport: responder}),
		withLimiter(fl),
		withSleep(sleepFn),
	}, extraOpts...)

	cfg := newConfig(opts)
	return cfg, fl, &sleeps
}

func newGetRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://example.invalid/test", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	return req
}

func TestDoRequest_SetsUserAgent(t *testing.T) {
	var gotUA string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return newTestResponse(http.StatusOK, "ok", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder, WithUserAgent("TestAgent/1.0"))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if gotUA != "TestAgent/1.0" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "TestAgent/1.0")
	}
}

func TestDoRequest_DefaultUserAgent(t *testing.T) {
	var gotUA string
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotUA = req.Header.Get("User-Agent")
		return newTestResponse(http.StatusOK, "ok", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder)

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if gotUA != DefaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, DefaultUserAgent)
	}
	if strings.Contains(gotUA, "@gmail.com") || strings.Contains(gotUA, "jodestic") {
		t.Errorf("default User-Agent must not contain a real personal email, got %q", gotUA)
	}
}

func TestDoRequest_RetriesOn429ThenSucceeds(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return newTestResponse(http.StatusTooManyRequests, "", nil), nil
		}
		return newTestResponse(http.StatusOK, "ok", nil), nil
	})

	cfg, limiter, sleeps := newTestConfig(t, responder, WithMaxRetries(3))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if limiter.waitCalls != 3 {
		t.Errorf("limiter.waitCalls = %d, want 3 (once per attempt)", limiter.waitCalls)
	}
	if len(*sleeps) != 2 {
		t.Errorf("sleeps = %v, want 2 backoff sleeps", *sleeps)
	}
}

func TestDoRequest_RetriesOn5xxThenSucceeds(t *testing.T) {
	attempts := 0
	statuses := []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable}
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if attempts < len(statuses) {
			s := statuses[attempts]
			attempts++
			return newTestResponse(s, "", nil), nil
		}
		attempts++
		return newTestResponse(http.StatusOK, "ok", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder, WithMaxRetries(3))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 4 {
		t.Errorf("attempts = %d, want 4", attempts)
	}
}

func TestDoRequest_DoesNotRetryOn400(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return newTestResponse(http.StatusBadRequest, "bad", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder, WithMaxRetries(3))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest returned error, want response with status 400: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 400)", attempts)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want 400", resp.StatusCode)
	}
}

func TestDoRequest_DoesNotRetryOn404(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return newTestResponse(http.StatusNotFound, "not found", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder, WithMaxRetries(3))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest returned error, want response with status 404: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 404)", attempts)
	}
}

func TestDoRequest_ExhaustsRetries429ReturnsErrRateLimited(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return newTestResponse(http.StatusTooManyRequests, "", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder, WithMaxRetries(2))

	_, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != ErrRateLimited {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (1 + 2 retries)", attempts)
	}
}

func TestDoRequest_ExhaustsRetries5xxReturnsErrSourceUnavailable(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return newTestResponse(http.StatusInternalServerError, "", nil), nil
	})

	cfg, _, _ := newTestConfig(t, responder, WithMaxRetries(2))

	_, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != ErrSourceUnavailable {
		t.Fatalf("err = %v, want ErrSourceUnavailable", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (1 + 2 retries)", attempts)
	}
}

func TestDoRequest_HonorsRetryAfterHeader(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return newTestResponse(http.StatusTooManyRequests, "", map[string]string{"Retry-After": "7"}), nil
		}
		return newTestResponse(http.StatusOK, "ok", nil), nil
	})

	cfg, _, sleeps := newTestConfig(t, responder, WithMaxRetries(3))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if len(*sleeps) != 1 {
		t.Fatalf("sleeps = %v, want exactly 1 recorded sleep", *sleeps)
	}
	if (*sleeps)[0] != 7*time.Second {
		t.Errorf("sleep duration = %v, want 7s (from Retry-After)", (*sleeps)[0])
	}
}

func TestDoRequest_RateLimiterConsultedOncePerAttempt(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 2 {
			return newTestResponse(http.StatusInternalServerError, "", nil), nil
		}
		return newTestResponse(http.StatusOK, "ok", nil), nil
	})

	cfg, limiter, _ := newTestConfig(t, responder, WithMaxRetries(3))

	resp, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if limiter.waitCalls != attempts {
		t.Errorf("limiter.waitCalls = %d, want %d (one per attempt)", limiter.waitCalls, attempts)
	}
}

func TestDoRequest_NetworkErrorRetriesThenExhausts(t *testing.T) {
	attempts := 0
	responder := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return nil, &fakeNetError{}
	})

	cfg, _, _ := newTestConfig(t, responder, WithMaxRetries(1))

	_, err := cfg.doRequest(context.Background(), newGetRequest(t))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "source unavailable") {
		t.Errorf("err = %v, want it to wrap ErrSourceUnavailable", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (1 + 1 retry)", attempts)
	}
}

// fakeNetError is a minimal error type simulating a transport-level failure
// (e.g. connection refused, timeout).
type fakeNetError struct{}

func (*fakeNetError) Error() string { return "simulated network error" }
