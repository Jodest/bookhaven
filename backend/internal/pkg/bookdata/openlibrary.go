package bookdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// defaultOpenLibraryBaseURL is the public Open Library REST API base URL.
const defaultOpenLibraryBaseURL = "https://openlibrary.org"

// OpenLibraryClient queries the Open Library REST API (Books API, Search
// API) and normalizes results into ExternalBookRecord. It is the enrichment
// source (cover, description, page count) per the catalog-data-import
// business analysis. Construct with NewOpenLibraryClient.
//
// OpenLibraryClient is point-lookup only (single ISBN, or single
// title+author search) — no bulk crawling loops live in this package, per
// Open Library's published etiquette (bulk retrieval belongs on their data
// dumps, not their live API).
type OpenLibraryClient struct {
	cfg *config
}

// Compile-time check that OpenLibraryClient satisfies Client.
var _ Client = (*OpenLibraryClient)(nil)

// NewOpenLibraryClient builds an OpenLibraryClient with the given options
// applied on top of sane defaults (public endpoint, DefaultUserAgent, ~1
// req/sec rate limit, retry-with-backoff on 429/5xx).
func NewOpenLibraryClient(opts ...Option) *OpenLibraryClient {
	cfg := newConfig(opts)
	if cfg.baseURL == "" {
		cfg.baseURL = defaultOpenLibraryBaseURL
	}
	return &OpenLibraryClient{cfg: cfg}
}

// SourceName returns "openlibrary".
func (c *OpenLibraryClient) SourceName() string { return "openlibrary" }

// FindByISBN looks up a single book by ISBN via the Open Library Books API
// (/api/books?bibkeys=ISBN:...&format=json&jscmd=data). Open Library
// returns HTTP 200 with an empty JSON object for an unknown ISBN (not a
// 404); this is explicitly mapped to ErrNotFound rather than an empty
// success.
func (c *OpenLibraryClient) FindByISBN(ctx context.Context, isbn string) (*ExternalBookRecord, error) {
	clean := normalizeISBNStructural(isbn)
	if clean == "" {
		return nil, ErrInvalidQuery
	}
	bibkey := "ISBN:" + clean

	u, err := url.Parse(c.cfg.baseURL + "/api/books")
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid open library base URL: %w", err)
	}
	q := u.Query()
	q.Set("bibkeys", bibkey)
	q.Set("format", "json")
	q.Set("jscmd", "data")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bookdata: build open library request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: open library returned status %d", ErrSourceUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bookdata: read open library response: %w", err)
	}

	var parsed map[string]olBookData
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("bookdata: parse open library response: %w", err)
	}

	data, ok := parsed[bibkey]
	if !ok {
		// Unknown ISBN: Open Library responds 200 with `{}`.
		return nil, ErrNotFound
	}

	return mapOLBookData(data, clean), nil
}

// FindByTitleAuthor looks up a single best-match book via the Open Library
// Search API (/search.json?title=...&author=...), returning the first
// (highest-ranked) result. Returns ErrNotFound if there are no matches.
func (c *OpenLibraryClient) FindByTitleAuthor(ctx context.Context, title, author string) (*ExternalBookRecord, error) {
	if strings.TrimSpace(title) == "" {
		return nil, ErrInvalidQuery
	}

	u, err := url.Parse(c.cfg.baseURL + "/search.json")
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid open library base URL: %w", err)
	}
	q := u.Query()
	q.Set("title", title)
	if strings.TrimSpace(author) != "" {
		q.Set("author", author)
	}
	q.Set("limit", "5")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bookdata: build open library request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: open library returned status %d", ErrSourceUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bookdata: read open library response: %w", err)
	}

	var parsed olSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("bookdata: parse open library search response: %w", err)
	}

	if len(parsed.Docs) == 0 {
		return nil, ErrNotFound
	}

	return mapOLSearchDoc(parsed.Docs[0]), nil
}

// CoverIDType identifies which kind of key is used to look up an Open
// Library cover image via the Covers API.
type CoverIDType string

const (
	// CoverIDTypeID identifies a numeric Open Library cover ID (e.g. from a
	// Search API doc's cover_i field).
	CoverIDTypeID CoverIDType = "id"
	// CoverIDTypeISBN identifies an ISBN-10 or ISBN-13.
	CoverIDTypeISBN CoverIDType = "isbn"
	// CoverIDTypeOLID identifies an Open Library edition ID (OLID).
	CoverIDTypeOLID CoverIDType = "olid"
)

// CoverSize selects the pixel dimensions of the returned cover image.
type CoverSize string

const (
	CoverSizeSmall  CoverSize = "S"
	CoverSizeMedium CoverSize = "M"
	CoverSizeLarge  CoverSize = "L"
)

// CoverURL builds an Open Library Covers API URL for the given id/idType/size
// without making a network call, matching the direct hotlink pattern already
// used in backend/cmd/seed (e.g.
// https://covers.openlibrary.org/b/id/12621906-L.jpg).
func CoverURL(id string, idType CoverIDType, size CoverSize) string {
	return fmt.Sprintf("https://covers.openlibrary.org/b/%s/%s-%s.jpg", idType, url.PathEscape(id), size)
}

// olBookData mirrors the relevant subset of the Open Library Books API
// (jscmd=data) response shape for a single bibkey.
type olBookData struct {
	Title         string        `json:"title"`
	Authors       []olNamed     `json:"authors"`
	Subjects      []olNamed     `json:"subjects"`
	PublishDate   string        `json:"publish_date"`
	NumberOfPages int           `json:"number_of_pages"`
	Cover         olCover       `json:"cover"`
	Identifiers   olIdentifiers `json:"identifiers"`
	Excerpts      []olExcerpt   `json:"excerpts"`
	Key           string        `json:"key"`
}

type olNamed struct {
	Name string `json:"name"`
}

type olCover struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

type olIdentifiers struct {
	ISBN10 []string `json:"isbn_10"`
	ISBN13 []string `json:"isbn_13"`
}

type olExcerpt struct {
	Text string `json:"text"`
}

// olSearchResponse mirrors the relevant subset of the Open Library Search
// API response shape.
type olSearchResponse struct {
	Docs []olSearchDoc `json:"docs"`
}

type olSearchDoc struct {
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	ISBN             []string `json:"isbn"`
	FirstPublishYear int      `json:"first_publish_year"`
	Subject          []string `json:"subject"`
	CoverI           int      `json:"cover_i"`
	Key              string   `json:"key"`
	Language         []string `json:"language"`
}

// openLibraryLanguageCodes is a small, best-effort map from Open Library's
// MARC-derived 3-letter language codes to ISO 639-1 2-letter codes, matching
// the 2-letter codes already used by the catalog's `language` field (see
// backend/cmd/seed). Unmapped codes fall back to the raw value.
var openLibraryLanguageCodes = map[string]string{
	"rus": "ru",
	"eng": "en",
	"ukr": "uk",
	"fre": "fr",
	"fra": "fr",
	"ger": "de",
	"deu": "de",
	"spa": "es",
	"ita": "it",
	"pol": "pl",
	"por": "pt",
}

func mapOpenLibraryLanguage(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if mapped, ok := openLibraryLanguageCodes[code]; ok {
		return mapped
	}
	return code
}

func namesOf(named []olNamed) []string {
	out := make([]string, 0, len(named))
	for _, n := range named {
		if n.Name != "" {
			out = append(out, n.Name)
		}
	}
	return out
}

var publishYearPattern = regexp.MustCompile(`\d{4}`)

func extractPublishYear(publishDate string) *int {
	match := publishYearPattern.FindString(publishDate)
	if match == "" {
		return nil
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return nil
	}
	return &year
}

// mapOLBookData converts a Books API bibkey entry into an ExternalBookRecord.
// requestedISBN is the (already structurally normalized) ISBN the caller
// looked up, used as a fallback when the response omits its own identifiers.
func mapOLBookData(data olBookData, requestedISBN string) *ExternalBookRecord {
	record := &ExternalBookRecord{
		Title:      data.Title,
		Authors:    namesOf(data.Authors),
		Genres:     namesOf(data.Subjects),
		SourceName: "openlibrary",
	}

	isbn := requestedISBN
	if len(data.Identifiers.ISBN13) > 0 {
		isbn = normalizeISBNStructural(data.Identifiers.ISBN13[0])
	} else if len(data.Identifiers.ISBN10) > 0 {
		isbn = normalizeISBNStructural(data.Identifiers.ISBN10[0])
	}
	if isbn != "" {
		record.ISBN = &isbn
	}

	if len(data.Excerpts) > 0 && data.Excerpts[0].Text != "" {
		desc := data.Excerpts[0].Text
		record.Description = &desc
	}

	switch {
	case data.Cover.Large != "":
		cover := data.Cover.Large
		record.CoverURL = &cover
	case data.Cover.Medium != "":
		cover := data.Cover.Medium
		record.CoverURL = &cover
	case data.Cover.Small != "":
		cover := data.Cover.Small
		record.CoverURL = &cover
	}

	if data.NumberOfPages > 0 {
		pages := data.NumberOfPages
		record.PageCount = &pages
	}

	if year := extractPublishYear(data.PublishDate); year != nil {
		record.PublishedYear = year
	}

	record.SourceID = lastPathSegment(data.Key)
	if record.SourceID == "" {
		record.SourceID = requestedISBN
	}

	return record
}

// mapOLSearchDoc converts a Search API doc into an ExternalBookRecord.
func mapOLSearchDoc(doc olSearchDoc) *ExternalBookRecord {
	// doc.AuthorName/doc.Subject are nil, not [], when a search result
	// simply lacks that field (common - many docs have no "subject" tag,
	// some have no "author_name" either). Authors/Genres must stay non-nil
	// so JSON marshaling produces [] rather than null - a null here has
	// broken the admin lookup-assistant UI in production (a frontend render
	// assumed .length is always callable on these fields, matching the
	// same non-nil guarantee namesOf already gives the FindByISBN path).
	authors := doc.AuthorName
	if authors == nil {
		authors = []string{}
	}
	genres := doc.Subject
	if genres == nil {
		genres = []string{}
	}
	record := &ExternalBookRecord{
		Title:      doc.Title,
		Authors:    authors,
		Genres:     genres,
		SourceName: "openlibrary",
		SourceID:   lastPathSegment(doc.Key),
	}

	if len(doc.ISBN) > 0 {
		isbn := normalizeISBNStructural(doc.ISBN[0])
		if isbn != "" {
			record.ISBN = &isbn
		}
	}

	if doc.FirstPublishYear > 0 {
		year := doc.FirstPublishYear
		record.PublishedYear = &year
	}

	if doc.CoverI > 0 {
		cover := CoverURL(strconv.Itoa(doc.CoverI), CoverIDTypeID, CoverSizeLarge)
		record.CoverURL = &cover
	}

	if len(doc.Language) > 0 {
		record.Language = mapOpenLibraryLanguage(doc.Language[0])
	}

	return record
}

func lastPathSegment(key string) string {
	key = strings.TrimSuffix(key, "/")
	idx := strings.LastIndex(key, "/")
	if idx == -1 {
		return key
	}
	return key[idx+1:]
}
