package bookdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// defaultWikidataSearchBaseURL is Wikidata's wbsearchentities action API
// endpoint - a plain JSON search API, distinct from the SPARQL endpoint
// (query.wikidata.org) used by Query/FindByISBN/FindByTitleAuthor/
// BuildAuthorListQuery elsewhere in this package.
const defaultWikidataSearchBaseURL = "https://www.wikidata.org/w/api.php"

// authorSearchLimit is a fixed MVP constant for SearchAuthors, not
// caller-configurable in this increment. Wikidata's own wbsearchentities
// default is 7, max 50; 10 is a reasonable balance between "enough to
// disambiguate" and "not an overwhelming list" for an admin picking a
// person by name.
const authorSearchLimit = 10

// AuthorCandidate is a single Wikidata item returned by SearchAuthors, for
// an admin to visually disambiguate before its QID is fed into
// BuildAuthorListQuery.
type AuthorCandidate struct {
	QID         string
	Label       string
	Description string // may be empty; Wikidata doesn't guarantee one
}

// wbSearchEntitiesResponse mirrors the relevant subset of Wikidata's
// wbsearchentities JSON response shape:
// {"searchinfo": {...}, "search": [{"id": "Q7200", "label": "...",
// "description": "...", "match": {"text": "..."}}], "success": 1}.
type wbSearchEntitiesResponse struct {
	Search []wbSearchEntityResult `json:"search"`
}

type wbSearchEntityResult struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Match       struct {
		Text string `json:"text"`
	} `json:"match"`
}

// SearchAuthors calls Wikidata's wbsearchentities action API
// (https://www.wikidata.org/w/api.php?action=wbsearchentities&...) to find
// candidate Wikidata items matching query by name. This is a different
// Wikidata HTTP API from the SPARQL endpoint used by every other method in
// this file - wbsearchentities is a plain JSON search API, no SPARQL
// involved - so it uses cfg.searchBaseURL (see WithSearchBaseURL), not
// cfg.baseURL.
//
// IMPORTANT / deliberate MVP scope decision: this method does NOT filter
// results to confirmed humans (wdt:P31 wd:Q5). wbsearchentities is typed to
// type=item only - there is no type=human option - so a non-person item
// (e.g. a book, a disambiguation page, an organization) sharing the
// searched name can appear in results. Callers/UI must treat every
// returned item as an unconfirmed candidate the admin has to visually vet
// via Description (Wikidata descriptions for people are almost always
// occupation-shaped, e.g. "русский писатель") before using its QID - see
// docs/features/harvest-author-search.md Business Rules/Risks for the full
// rationale (adding a server-side P31/Q5 check would double the outbound
// Wikidata calls per search, add latency to a synchronous admin-facing
// lookup, and still not fully resolve ambiguity between multiple humans
// sharing a name).
//
// query is trimmed; an empty query returns ErrInvalidQuery without making
// an HTTP call. Zero matches is a normal, successful result: SearchAuthors
// returns an empty slice and a nil error, not ErrNotFound - this is a
// list/search endpoint where "no results" is a normal, displayable state
// (unlike FindByISBN/FindByTitleAuthor, which model a single point lookup).
func (c *WikidataClient) SearchAuthors(ctx context.Context, query string) ([]AuthorCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrInvalidQuery
	}

	u, err := url.Parse(c.cfg.searchBaseURL)
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid wikidata search base URL: %w", err)
	}
	q := u.Query()
	q.Set("action", "wbsearchentities")
	q.Set("search", query)
	q.Set("language", "ru")
	q.Set("uselang", "ru")
	q.Set("type", "item")
	q.Set("format", "json")
	q.Set("limit", strconv.Itoa(authorSearchLimit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bookdata: build wikidata search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

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
		return nil, fmt.Errorf("bookdata: read wikidata search response: %w", err)
	}

	var parsed wbSearchEntitiesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("bookdata: parse wikidata search response: %w", err)
	}

	candidates := make([]AuthorCandidate, 0, len(parsed.Search))
	for _, r := range parsed.Search {
		label := r.Label
		if label == "" {
			label = r.Match.Text
		}
		candidates = append(candidates, AuthorCandidate{
			QID:         r.ID,
			Label:       label,
			Description: r.Description,
		})
	}
	return candidates, nil
}
