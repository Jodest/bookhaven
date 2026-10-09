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

// defaultWikidataBaseURL is the public Wikidata SPARQL query endpoint.
const defaultWikidataBaseURL = "https://query.wikidata.org/sparql"

// WikidataClient queries the public Wikidata SPARQL endpoint and normalizes
// results into ExternalBookRecord. It is the source of structured facts
// (title, authors, genre, published year) per the catalog-data-import
// business analysis. Construct with NewWikidataClient.
type WikidataClient struct {
	cfg *config
}

// Compile-time check that WikidataClient satisfies Client.
var _ Client = (*WikidataClient)(nil)

// NewWikidataClient builds a WikidataClient with the given options applied
// on top of sane defaults (public endpoint, DefaultUserAgent, ~1 req/sec
// rate limit, retry-with-backoff on 429/5xx).
func NewWikidataClient(opts ...Option) *WikidataClient {
	cfg := newConfig(opts)
	if cfg.baseURL == "" {
		cfg.baseURL = defaultWikidataBaseURL
	}
	if cfg.searchBaseURL == "" {
		cfg.searchBaseURL = defaultWikidataSearchBaseURL
	}
	if cfg.commonsBaseURL == "" {
		cfg.commonsBaseURL = defaultCommonsBaseURL
	}
	return &WikidataClient{cfg: cfg}
}

// SourceName returns "wikidata".
func (c *WikidataClient) SourceName() string { return "wikidata" }

// Query executes an arbitrary, caller-supplied SPARQL query against the
// configured endpoint and parses the standard SPARQL JSON results format
// (results.bindings[]) into normalized records. Query is a bulk-capable
// method not on the shared Client interface — it is the intended entry
// point for the future cmd/harvest increment.
//
// Query expects (but does not require the caller to alias exactly) the
// following result variables, matching what buildTitleAuthorQuery and
// buildISBNQuery produce: item, itemLabel, authors (pipe-separated
// "<QID>\t<label>" pairs; a bare label without a tab is accepted as a
// QID-less name), genres (same pipe-separated "<QID>\t<label>" pairing as
// authors), isbn, published (a date literal), language (an English
// language name label). Missing optional variables simply leave the
// corresponding ExternalBookRecord field unset.
func (c *WikidataClient) Query(ctx context.Context, sparql string) ([]ExternalBookRecord, error) {
	bindings, err := c.sparqlBindings(ctx, sparql)
	if err != nil {
		return nil, err
	}
	return parseSparqlBindings(bindings), nil
}

// sparqlBindings runs one SPARQL query on cfg.baseURL via cfg.doRequest and
// returns the raw results.bindings. Shared by Query and ClassifyGenres. An
// empty query returns ErrInvalidQuery without a call; HTTP 400 →
// ErrInvalidQuery, other >= 400 → ErrSourceUnavailable (429/5xx are
// handled by doRequest); malformed JSON is a wrapped parse error.
func (c *WikidataClient) sparqlBindings(ctx context.Context, sparql string) ([]map[string]sparqlValue, error) {
	if strings.TrimSpace(sparql) == "" {
		return nil, ErrInvalidQuery
	}

	u, err := url.Parse(c.cfg.baseURL)
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid wikidata base URL: %w", err)
	}
	q := u.Query()
	q.Set("query", sparql)
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bookdata: build wikidata request: %w", err)
	}
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := c.cfg.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidQuery
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: wikidata returned status %d", ErrSourceUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bookdata: read wikidata response: %w", err)
	}

	var parsed sparqlResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("bookdata: parse wikidata response: %w", err)
	}
	return parsed.Results.Bindings, nil
}

// FindByISBN looks up a single book by ISBN-10 or ISBN-13 (Wikidata
// properties P957 and P212 respectively). Returns ErrNotFound if Wikidata
// has no matching item.
func (c *WikidataClient) FindByISBN(ctx context.Context, isbn string) (*ExternalBookRecord, error) {
	clean := normalizeISBNStructural(isbn)
	if clean == "" {
		return nil, ErrInvalidQuery
	}
	records, err := c.Query(ctx, buildISBNQuery(clean))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, ErrNotFound
	}
	return &records[0], nil
}

// FindByTitleAuthor builds an internal, parameterized SPARQL query for a
// point lookup by title and (optionally empty) author, and returns the
// first result. Returns ErrNotFound if Wikidata has no matching item.
func (c *WikidataClient) FindByTitleAuthor(ctx context.Context, title, author string) (*ExternalBookRecord, error) {
	if strings.TrimSpace(title) == "" {
		return nil, ErrInvalidQuery
	}
	records, err := c.Query(ctx, buildTitleAuthorQuery(title, author))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, ErrNotFound
	}
	return &records[0], nil
}

// buildTitleAuthorQuery constructs a SPARQL query that finds literary works
// whose Russian (`ru`) label contains title, optionally constrained to
// items with a matching Russian author label. Restricting to `ru` labels
// (rather than falling back to `en`) is a deliberate "Russian-only catalog
// content" decision (see docs/features/russian-only-content.md): any work,
// author, or genre with no Russian-language Wikidata label is now skipped
// entirely rather than harvested under an English label - an accepted
// reduction in harvestable coverage in exchange for never harvesting
// non-Russian catalog text in the first place.
func buildTitleAuthorQuery(title, author string) string {
	authorFilter := ""
	if strings.TrimSpace(author) != "" {
		authorFilter = fmt.Sprintf(`
    ?item wdt:P50 ?author .
    ?author rdfs:label ?authorLabel .
    FILTER(LANG(?authorLabel) = "ru")
    FILTER(CONTAINS(LCASE(?authorLabel), LCASE("%s")))`, escapeSPARQLString(author))
	}

	return fmt.Sprintf(`SELECT ?item ?itemLabel
  (GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?author), "/entity/"), "\t", ?authorLabel); separator="|") AS ?authors)
  (GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?genre), "/entity/"), "\t", ?genreLabel); separator="|") AS ?genres)
  (SAMPLE(?isbn13) AS ?isbn)
  (SAMPLE(?pubDate) AS ?published)
  (SAMPLE(?languageLabel) AS ?language)
WHERE {
  ?item wdt:P31/wdt:P279* wd:Q7725634 .
  ?item rdfs:label ?itemLabel .
  FILTER(LANG(?itemLabel) = "ru")
  FILTER(CONTAINS(LCASE(?itemLabel), LCASE("%s")))
  OPTIONAL {%s
  }
  OPTIONAL {
    ?item wdt:P136 ?genre .
    ?genre rdfs:label ?genreLabel .
    FILTER(LANG(?genreLabel) = "ru")
  }
  OPTIONAL { ?item wdt:P212 ?isbn13 . }
  OPTIONAL { ?item wdt:P577 ?pubDate . }
  OPTIONAL {
    ?item wdt:P407 ?language .
    ?language rdfs:label ?languageLabel .
    FILTER(LANG(?languageLabel) = "en")
  }
}
GROUP BY ?item ?itemLabel
LIMIT 5`, escapeSPARQLString(title), authorFilter)
}

// buildISBNQuery constructs a SPARQL query that finds the literary work
// edition matching the given (already structurally-normalized) ISBN via
// either P212 (ISBN-13) or P957 (ISBN-10). Like buildTitleAuthorQuery, item/
// author/genre labels are restricted to `ru` only (no `en` fallback) - see
// that function's doc comment for the rationale and accepted coverage
// tradeoff.
func buildISBNQuery(isbn string) string {
	escaped := escapeSPARQLString(isbn)
	return fmt.Sprintf(`SELECT ?item ?itemLabel
  (GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?author), "/entity/"), "\t", ?authorLabel); separator="|") AS ?authors)
  (GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?genre), "/entity/"), "\t", ?genreLabel); separator="|") AS ?genres)
  (SAMPLE(?isbn13) AS ?isbn)
  (SAMPLE(?pubDate) AS ?published)
  (SAMPLE(?languageLabel) AS ?language)
WHERE {
  { ?item wdt:P212 "%s" . } UNION { ?item wdt:P957 "%s" . }
  ?item rdfs:label ?itemLabel .
  FILTER(LANG(?itemLabel) = "ru")
  OPTIONAL {
    ?item wdt:P50 ?author .
    ?author rdfs:label ?authorLabel .
    FILTER(LANG(?authorLabel) = "ru")
  }
  OPTIONAL {
    ?item wdt:P136 ?genre .
    ?genre rdfs:label ?genreLabel .
    FILTER(LANG(?genreLabel) = "ru")
  }
  OPTIONAL { ?item wdt:P212 ?isbn13 . }
  OPTIONAL { ?item wdt:P577 ?pubDate . }
  OPTIONAL {
    ?item wdt:P407 ?language .
    ?language rdfs:label ?languageLabel .
    FILTER(LANG(?languageLabel) = "en")
  }
}
GROUP BY ?item ?itemLabel
LIMIT 5`, escaped, escaped)
}

// wikidataQIDPattern validates a bare Wikidata entity QID (no "wd:" prefix,
// case-sensitive), e.g. "Q7200".
var wikidataQIDPattern = regexp.MustCompile(`^Q[0-9]+$`)

const (
	// defaultAuthorListLimit is applied by BuildAuthorListQuery when the
	// caller passes limit <= 0.
	defaultAuthorListLimit = 500
	// maxAuthorListLimit is the upper bound BuildAuthorListQuery will honor;
	// requested limits above this are clamped down to it rather than
	// rejected, matching the product owner's own stated "100 to 5000" range
	// for a single harvest job's result set.
	maxAuthorListLimit = 5000
)

// BuildAuthorListQuery constructs a bounded SPARQL query for "Approach A"
// curated-author-list harvesting (see
// docs/features/harvest-review-queue.md): it finds literary works (the same
// wd:Q7725634 class filter used by buildTitleAuthorQuery/buildISBNQuery)
// authored (wdt:P50) by any of the given Wikidata author QIDs, constrained
// by a VALUES clause built from those QIDs, and reuses the same `ru`-only
// label-filter idiom for both the work and the author (see
// buildTitleAuthorQuery's doc comment for the rationale and accepted
// coverage tradeoff). Output variables (item, itemLabel, authors, genres,
// isbn, published, language) match what parseSparqlBindings already
// expects.
//
// authorQIDs must be bare QIDs (no "wd:" prefix - this function adds it),
// case-sensitive, matching ^Q[0-9]+$. Entries are deduped. An empty list or
// any malformed QID returns an error wrapping ErrInvalidQuery.
//
// limit <= 0 applies a default of 500; a requested limit above 5000 is
// clamped down to 5000 (not rejected) to keep a single harvest job's result
// set - and the ~1 req/sec Open-Library-enrichment tail that follows in the
// worker - bounded. limit bounds the query's LIMIT, not the number of
// author QIDs.
func BuildAuthorListQuery(authorQIDs []string, limit int) (string, error) {
	seen := make(map[string]struct{}, len(authorQIDs))
	values := make([]string, 0, len(authorQIDs))
	for _, qid := range authorQIDs {
		qid = strings.TrimSpace(qid)
		if qid == "" {
			continue
		}
		if !wikidataQIDPattern.MatchString(qid) {
			return "", fmt.Errorf("%w: invalid author QID %q (expected e.g. \"Q7200\")", ErrInvalidQuery, qid)
		}
		if _, ok := seen[qid]; ok {
			continue
		}
		seen[qid] = struct{}{}
		values = append(values, "wd:"+qid)
	}
	if len(values) == 0 {
		return "", fmt.Errorf("%w: at least one author QID is required", ErrInvalidQuery)
	}

	if limit <= 0 {
		limit = defaultAuthorListLimit
	}
	if limit > maxAuthorListLimit {
		limit = maxAuthorListLimit
	}

	return fmt.Sprintf(`SELECT ?item ?itemLabel
  (GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?author), "/entity/"), "\t", ?authorLabel); separator="|") AS ?authors)
  (GROUP_CONCAT(DISTINCT CONCAT(STRAFTER(STR(?genre), "/entity/"), "\t", ?genreLabel); separator="|") AS ?genres)
  (SAMPLE(?isbn13) AS ?isbn)
  (SAMPLE(?pubDate) AS ?published)
  (SAMPLE(?languageLabel) AS ?language)
WHERE {
  VALUES ?author { %s }
  ?item wdt:P31/wdt:P279* wd:Q7725634 .
  ?item wdt:P50 ?author .
  ?item rdfs:label ?itemLabel .
  FILTER(LANG(?itemLabel) = "ru")
  ?author rdfs:label ?authorLabel .
  FILTER(LANG(?authorLabel) = "ru")
  OPTIONAL {
    ?item wdt:P136 ?genre .
    ?genre rdfs:label ?genreLabel .
    FILTER(LANG(?genreLabel) = "ru")
  }
  OPTIONAL { ?item wdt:P212 ?isbn13 . }
  OPTIONAL { ?item wdt:P577 ?pubDate . }
  OPTIONAL {
    ?item wdt:P407 ?language .
    ?language rdfs:label ?languageLabel .
    FILTER(LANG(?languageLabel) = "en")
  }
}
GROUP BY ?item ?itemLabel
LIMIT %d`, strings.Join(values, " "), limit), nil
}

// escapeSPARQLString escapes a Go string for safe embedding inside a
// double-quoted SPARQL string literal.
func escapeSPARQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// sparqlResponse mirrors the standard SPARQL 1.1 JSON results format:
// https://www.w3.org/TR/sparql11-results-json/
type sparqlResponse struct {
	Results struct {
		Bindings []map[string]sparqlValue `json:"bindings"`
	} `json:"results"`
}

type sparqlValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Lang  string `json:"xml:lang,omitempty"`
}

var wikidataEntityIDPattern = regexp.MustCompile(`([A-Za-z][0-9]+)$`)

// wikidataLanguageCodes is a small, best-effort map from the English
// language name Wikidata returns (via rdfs:label on the P407 value) to an
// ISO 639-1 code. Unmapped languages fall back to an empty Language field
// rather than guessing.
var wikidataLanguageCodes = map[string]string{
	"Russian":    "ru",
	"English":    "en",
	"French":     "fr",
	"German":     "de",
	"Spanish":    "es",
	"Ukrainian":  "uk",
	"Polish":     "pl",
	"Italian":    "it",
	"Portuguese": "pt",
}

// parseSparqlBindings converts SPARQL JSON result bindings into
// ExternalBookRecord values. One binding produces one record; rows with a
// missing "item" or "itemLabel" value are skipped as unusable.
func parseSparqlBindings(bindings []map[string]sparqlValue) []ExternalBookRecord {
	records := make([]ExternalBookRecord, 0, len(bindings))
	for _, binding := range bindings {
		itemValue, ok := binding["item"]
		if !ok || itemValue.Value == "" {
			continue
		}
		titleValue, ok := binding["itemLabel"]
		if !ok || titleValue.Value == "" {
			continue
		}

		record := ExternalBookRecord{
			Title:      titleValue.Value,
			SourceName: "wikidata",
			SourceID:   extractWikidataID(itemValue.Value),
		}

		if v, ok := binding["authors"]; ok && v.Value != "" {
			record.AuthorRefs = parseAuthorPairs(v.Value)
			record.Authors = make([]string, 0, len(record.AuthorRefs))
			for _, ref := range record.AuthorRefs {
				record.Authors = append(record.Authors, ref.Name)
			}
		}
		if v, ok := binding["genres"]; ok && v.Value != "" {
			record.GenreRefs = parseGenrePairs(v.Value)
			record.Genres = make([]string, 0, len(record.GenreRefs))
			for _, ref := range record.GenreRefs {
				record.Genres = append(record.Genres, ref.Name)
			}
		}
		if v, ok := binding["isbn"]; ok && v.Value != "" {
			isbn := normalizeISBNStructural(v.Value)
			if isbn != "" {
				record.ISBN = &isbn
			}
		}
		if v, ok := binding["published"]; ok && v.Value != "" {
			if year := parseYearFromDate(v.Value); year != nil {
				record.PublishedYear = year
			}
		}
		if v, ok := binding["language"]; ok && v.Value != "" {
			if code, known := wikidataLanguageCodes[v.Value]; known {
				record.Language = code
			}
		}

		records = append(records, record)
	}
	return records
}

// extractWikidataID extracts the trailing entity ID (e.g. "Q170583") from a
// Wikidata entity URI.
func extractWikidataID(uri string) string {
	if match := wikidataEntityIDPattern.FindString(uri); match != "" {
		return match
	}
	return uri
}

// qidPair is one entry of a paired "<QID>\t<label>" aggregate.
type qidPair struct {
	QID  string
	Name string
}

// parseQIDPairs parses a paired aggregate produced by the query builders
// (authors and genres): entries separated by "|", each entry
// "<QID>\t<label>". Each entry is split on its first tab only, so a label
// can never shift into another entry's QID. An entry with no tab is treated
// as a bare name (QID empty); a malformed QID is dropped while the name is
// kept; entries with an empty name are skipped.
func parseQIDPairs(value string) []qidPair {
	parts := strings.Split(value, "|")
	out := make([]qidPair, 0, len(parts))
	for _, p := range parts {
		qid, name := "", p
		if i := strings.Index(p, "\t"); i >= 0 {
			qid, name = strings.TrimSpace(p[:i]), p[i+1:]
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !wikidataQIDPattern.MatchString(qid) {
			qid = ""
		}
		out = append(out, qidPair{QID: qid, Name: name})
	}
	return out
}

// parseAuthorPairs parses the paired author aggregate (see parseQIDPairs).
func parseAuthorPairs(value string) []ExternalAuthor {
	pairs := parseQIDPairs(value)
	out := make([]ExternalAuthor, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, ExternalAuthor{Name: p.Name, QID: p.QID})
	}
	return out
}

// parseGenrePairs parses the paired genre aggregate (see parseQIDPairs).
func parseGenrePairs(value string) []ExternalGenre {
	pairs := parseQIDPairs(value)
	out := make([]ExternalGenre, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, ExternalGenre{Name: p.Name, QID: p.QID})
	}
	return out
}

var yearPattern = regexp.MustCompile(`^-?(\d{1,4})`)

// parseYearFromDate extracts a year from a date literal such as
// "1869-01-01T00:00:00Z" (the xsd:dateTime shape Wikidata typically
// returns for P577/publication date).
func parseYearFromDate(value string) *int {
	match := yearPattern.FindString(value)
	if match == "" {
		return nil
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return nil
	}
	return &year
}
