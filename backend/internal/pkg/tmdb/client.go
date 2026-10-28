// Package tmdb is a minimal TMDB API client that resolves a film or series
// poster URL (openspec change add-adaptation-posters-and-tabs).
//
// Every error message is built without the request URL, so the API key never
// reaches logs.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultBaseURL = "https://api.themoviedb.org/3"
	// ImageBaseURL is the TMDB image CDN path of a 500 px wide poster.
	ImageBaseURL   = "https://image.tmdb.org/t/p/w500"
	requestTimeout = 10 * time.Second
	maxBodyBytes   = 64 << 10
	requestsPerSec = 10
)

// Kinds accepted by Poster.
const (
	KindMovie = "movie"
	KindTV    = "tv"
)

var (
	// ErrUnauthorized: TMDB rejected the API key (401).
	ErrUnauthorized = errors.New("tmdb: unauthorized")
	// ErrUnavailable: rate limited, server error or network failure.
	ErrUnavailable = errors.New("tmdb: unavailable")
	// ErrInvalidQuery: unsupported kind or malformed id; no call is made.
	ErrInvalidQuery = errors.New("tmdb: invalid query")

	posterPathRe = regexp.MustCompile(`^/[A-Za-z0-9_-]+\.(jpg|jpeg|png)$`)
	idRe         = regexp.MustCompile(`^[0-9]{1,10}$`)
)

// Client calls the TMDB API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
	limiter *rate.Limiter
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (tests).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient overrides the HTTP client (tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New creates a Client for the API key, paced at 10 requests per second.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: requestTimeout},
		limiter: rate.NewLimiter(rate.Limit(requestsPerSec), 1),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type detailResponse struct {
	PosterPath *string `json:"poster_path"`
}

// Poster returns the poster image URL of a movie or series, or "" when TMDB
// has no poster (or no such id). kind is KindMovie or KindTV.
func (c *Client) Poster(ctx context.Context, kind, id string) (string, error) {
	if (kind != KindMovie && kind != KindTV) || !idRe.MatchString(id) {
		return "", ErrInvalidQuery
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, ctx.Err())
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	q := url.Values{}
	q.Set("language", "ru-RU")
	q.Set("api_key", c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+kind+"/"+id+"?"+q.Encode(), nil)
	if err != nil {
		return "", ErrUnavailable
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// *url.Error carries the URL with the key: drop it.
		return "", ErrUnavailable
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return "", nil
	case resp.StatusCode == http.StatusUnauthorized:
		return "", ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	var out detailResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&out); err != nil {
		return "", fmt.Errorf("%w: invalid response", ErrUnavailable)
	}
	if out.PosterPath == nil || !posterPathRe.MatchString(*out.PosterPath) {
		return "", nil
	}
	return ImageBaseURL + *out.PosterPath, nil
}
