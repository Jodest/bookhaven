package bookdata

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// DefaultUserAgent is sent on every outbound request unless overridden via
// WithUserAgent. The contact value below is a PLACEHOLDER — replace it with
// a real, monitored contact (project URL or issue tracker, not a personal
// email address) before using this package against production traffic. See
// the package doc comment for details.
const DefaultUserAgent = "BookHaven/0.1 (+https://github.com/bookhaven; contact: bookhaven-project-placeholder@example.invalid)"

const (
	defaultMaxRetries     = 3
	defaultRetryBaseDelay = 500 * time.Millisecond
	defaultRetryMaxDelay  = 10 * time.Second
	defaultRateLimit      = 1.0 // requests per second, per client instance
	defaultBurst          = 1
	defaultTimeout        = 15 * time.Second
)

// limiter is the minimal rate-limiting surface consumed by doRequest. It is
// satisfied by *golang.org/x/time/rate.Limiter (the production default) and
// by fake, instant limiters in tests.
type limiter interface {
	Wait(ctx context.Context) error
}

// config holds the shared, unexported HTTP plumbing used by both
// WikidataClient and OpenLibraryClient. It is built by newConfig and
// customized via functional Options.
type config struct {
	httpClient     *http.Client
	userAgent      string
	baseURL        string
	limiter        limiter
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
	sleep          func(time.Duration)
}

// Option configures a WikidataClient or OpenLibraryClient at construction
// time.
type Option func(*config)

// WithHTTPClient overrides the underlying *http.Client (default: a client
// with a 15s timeout).
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *config) { cfg.httpClient = c }
}

// WithUserAgent overrides the User-Agent header sent on every request
// (default: DefaultUserAgent).
func WithUserAgent(userAgent string) Option {
	return func(cfg *config) { cfg.userAgent = userAgent }
}

// WithBaseURL overrides the source's base URL, primarily so tests can point
// a client at a local httptest server instead of the real public endpoint.
func WithBaseURL(baseURL string) Option {
	return func(cfg *config) { cfg.baseURL = baseURL }
}

// WithRateLimit overrides the outbound request rate limiter (default: ~1
// request/second, burst 1, per client instance).
func WithRateLimit(requestsPerSecond float64, burst int) Option {
	return func(cfg *config) {
		cfg.limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
	}
}

// WithMaxRetries overrides the maximum number of retry attempts after the
// initial request (default: 3).
func WithMaxRetries(n int) Option {
	return func(cfg *config) { cfg.maxRetries = n }
}

// withLimiter substitutes a fake limiter. Unexported: only used by this
// package's own tests, which live in the same package.
func withLimiter(l limiter) Option {
	return func(cfg *config) { cfg.limiter = l }
}

// withSleep substitutes the backoff sleep function so tests never perform a
// real wall-clock wait. Unexported: only used by this package's own tests.
func withSleep(sleep func(time.Duration)) Option {
	return func(cfg *config) { cfg.sleep = sleep }
}

// newConfig builds a config with defaults applied, then applies opts.
func newConfig(opts []Option) *config {
	cfg := &config{
		httpClient:     &http.Client{Timeout: defaultTimeout},
		userAgent:      DefaultUserAgent,
		limiter:        rate.NewLimiter(rate.Limit(defaultRateLimit), defaultBurst),
		maxRetries:     defaultMaxRetries,
		retryBaseDelay: defaultRetryBaseDelay,
		retryMaxDelay:  defaultRetryMaxDelay,
		sleep:          time.Sleep,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// backoffDelay returns the exponential backoff delay for the given zero-based
// retry attempt number, capped at retryMaxDelay.
func (cfg *config) backoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(cfg.retryBaseDelay) * math.Pow(2, float64(attempt)))
	if delay > cfg.retryMaxDelay {
		delay = cfg.retryMaxDelay
	}
	return delay
}

// parseRetryAfter parses an HTTP Retry-After header value, which may be
// either an integer number of seconds or an HTTP-date. Returns 0 if the
// header is absent or unparsable, signaling "use exponential backoff
// instead".
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

// doRequest sends req, applying etiquette (User-Agent), throttling (rate
// limiter token per attempt), and retry-with-backoff on HTTP 429/5xx and
// transport errors, up to cfg.maxRetries additional attempts beyond the
// first.
//
// On success, or on a non-retryable 4xx response, the *http.Response is
// returned as-is (status code included) for the caller to interpret —
// e.g. 404 often means "not found" for one source but "bad ISBN" for
// another, so that mapping is left to wikidata.go / openlibrary.go.
//
// If retries are exhausted, doRequest returns (nil, ErrRateLimited) when the
// terminal condition was 429s, (nil, ErrSourceUnavailable) when it was 5xx
// responses or transport errors, and never returns both a non-nil response
// and a non-nil error.
func (cfg *config) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", cfg.userAgent)

	var lastErr error
	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		if err := cfg.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("bookdata: rate limiter wait: %w", err)
		}

		attemptReq := req.Clone(ctx)

		resp, err := cfg.httpClient.Do(attemptReq)
		if err != nil {
			lastErr = fmt.Errorf("%w: %v", ErrSourceUnavailable, err)
			if attempt == cfg.maxRetries {
				return nil, lastErr
			}
			cfg.sleep(cfg.backoffDelay(attempt))
			continue
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			drainAndClose(resp)
			lastErr = ErrRateLimited
			if attempt == cfg.maxRetries {
				return nil, ErrRateLimited
			}
			delay := retryAfter
			if delay <= 0 {
				delay = cfg.backoffDelay(attempt)
			}
			cfg.sleep(delay)
			continue

		case resp.StatusCode >= 500:
			drainAndClose(resp)
			lastErr = ErrSourceUnavailable
			if attempt == cfg.maxRetries {
				return nil, ErrSourceUnavailable
			}
			cfg.sleep(cfg.backoffDelay(attempt))
			continue

		default:
			// Success (< 300) or a non-retryable 4xx: return as-is.
			return resp, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrSourceUnavailable
}

func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
