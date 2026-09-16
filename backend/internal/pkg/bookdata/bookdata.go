// Package bookdata provides HTTP clients for external, open book-data
// sources so future BookHaven features (a bulk `cmd/harvest` CLI and an
// admin single-lookup endpoint, both planned but not part of this package)
// can look up book metadata without embedding source-specific HTTP/parsing
// logic of their own.
//
// Two sources are supported, matching the roles assigned to them by the
// catalog-data-import business analysis (see
// docs/ai/features/catalog-data-import/business-analysis.md):
//
//   - WikidataClient talks to the public Wikidata SPARQL endpoint
//     (query.wikidata.org) and is the source of structured facts: title,
//     authors, genre, published year. It supports both an arbitrary
//     caller-supplied SPARQL query (Query) for bulk retrieval and a
//     built-in title(+author) point lookup via the shared Client interface.
//   - OpenLibraryClient talks to the Open Library REST API
//     (openlibrary.org) and is the enrichment source: cover, description,
//     page count. It is a point-lookup-only client (single ISBN, or single
//     title+author search) — Open Library's etiquette reserves bulk
//     retrieval for their published data dumps, not their live API, so this
//     package deliberately does not offer a bulk method for it.
//
// Both clients implement the common Client interface and return results as
// ExternalBookRecord, a normalized shape that is field-compatible (by
// shape, not by Go import — this package does not depend on
// internal/modules/admin) with admin/service.BookInput. This package does
// not decide which source "wins" for a given field when both are queried
// for the same book; that merge policy belongs to the future cmd/harvest
// increment. It also does not perform ISBN checksum validation (that stays
// owned by admin/service.isValidISBN, applied when a record is fed into
// admin.Service.CreateBook) — ISBN values here are only structurally
// normalized (trimmed, dashes/spaces stripped).
//
// # Etiquette
//
// Both public APIs are free services operated on a "please be reasonable"
// basis. Every outbound request sets a descriptive User-Agent (see
// DefaultUserAgent) and is throttled through a shared rate limiter with
// retry-with-backoff on HTTP 429/5xx (see transport.go). Callers should not
// bypass this by constructing raw *http.Client calls against these APIs.
//
// IMPORTANT: DefaultUserAgent contains a placeholder contact value. Replace
// it (via WithUserAgent) with a real, monitored contact (a project URL or
// issue tracker — not an individual's personal email address, since the
// User-Agent is a public, third-party-visible HTTP header) before this
// package is used against production/high-volume traffic.
//
// This package has no live caller yet in this increment; it ships with its
// own unit tests (using a fake http.RoundTripper — no real network calls)
// and is not wired into any HTTP route, CLI command, or database migration.
package bookdata

import (
	"context"
	"errors"
)

// Sentinel errors returned by both clients. Callers should use errors.Is
// for comparison since transport-level wrapping may add context.
var (
	// ErrNotFound indicates a point lookup (FindByISBN, FindByTitleAuthor)
	// found no matching record at the source.
	ErrNotFound = errors.New("bookdata: record not found")

	// ErrRateLimited indicates the source responded with HTTP 429 on every
	// retry attempt and the request was abandoned.
	ErrRateLimited = errors.New("bookdata: rate limited by source")

	// ErrSourceUnavailable indicates the source responded with HTTP 5xx (or
	// a transport/network error) on every retry attempt and the request was
	// abandoned.
	ErrSourceUnavailable = errors.New("bookdata: source unavailable")

	// ErrInvalidQuery indicates the request was rejected as malformed
	// (e.g. HTTP 400, or an empty/invalid query built locally) and was not
	// retried.
	ErrInvalidQuery = errors.New("bookdata: invalid query")
)

// ExternalBookRecord is the normalized shape returned by both WikidataClient
// and OpenLibraryClient for a single book. Fields are semantically
// compatible with admin/service.BookInput, except Authors and Genres are
// names here (not internal database IDs) since resolving names to IDs is
// the responsibility of a later increment (cmd/harvest / the admin lookup
// endpoint), not this package.
type ExternalBookRecord struct {
	// Title is the book's title as reported by the source.
	Title string
	// Authors are author display names as reported by the source.
	Authors []string
	// Genres are genre/subject names as reported by the source.
	Genres []string
	// ISBN is structurally normalized (trimmed, dashes/spaces stripped)
	// but not checksum-validated. Nil if the source did not report one.
	ISBN *string
	// Description is a free-text summary, when the source provides one.
	Description *string
	// CoverURL is a direct, hotlinkable cover image URL, when available.
	CoverURL *string
	// PublishedYear is the first/original publication year, when known.
	PublishedYear *int
	// PageCount is the page count of the edition the source reported, when known.
	PageCount *int
	// Language is a best-effort language code (e.g. "ru", "en"); may be
	// empty if the source did not report a resolvable language.
	Language string
	// SourceName identifies which client produced this record, e.g.
	// "wikidata" or "openlibrary".
	SourceName string
	// SourceID is the source's own identifier for the record (e.g. a
	// Wikidata QID, or an Open Library edition/work key), for traceability.
	SourceID string
}

// Client is the shared point-lookup interface implemented by both
// WikidataClient and OpenLibraryClient. It intentionally excludes bulk
// retrieval: WikidataClient.Query is exposed separately since Open Library's
// etiquette reserves bulk retrieval for its data dumps, not its live API.
type Client interface {
	// SourceName returns the stable identifier of the underlying source,
	// e.g. "wikidata" or "openlibrary".
	SourceName() string
	// FindByISBN looks up a single book by ISBN (10 or 13 digit, dashes
	// tolerated). Returns ErrNotFound if the source has no match.
	FindByISBN(ctx context.Context, isbn string) (*ExternalBookRecord, error)
	// FindByTitleAuthor looks up a single best-match book by title and
	// (optionally empty) author. Returns ErrNotFound if the source has no
	// match.
	FindByTitleAuthor(ctx context.Context, title, author string) (*ExternalBookRecord, error)
}
