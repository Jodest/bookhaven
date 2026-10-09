package bookdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// wikidataEntityMaxBytes bounds the wbgetentities response body read by
// FetchAuthor. Famous entities carry large claim sets (hundreds of KB), so
// the cap is generous; it only guards against a runaway response.
const wikidataEntityMaxBytes = 8 << 20 // 8 MiB

// ExternalAuthorRecord is a single Wikidata author entity fetched by
// FetchAuthor for a direct author import (docs/features/author-direct-import.md).
//
// The enrichment fields (dates, PhotoFile) come from the same wbgetentities
// response (docs/features/author-wikidata-enrichment.md).
type ExternalAuthorRecord struct {
	// QID is the returned entity's id - canonical after a redirect, so it
	// may differ from the requested QID.
	QID string
	// Name is the entity's Russian ("ru") label, or "" when it has none.
	// No language fallback is applied.
	Name string
	// InstanceOf lists the valid QIDs of the entity's P31 (instance of)
	// claims; novalue/somevalue snaks are skipped.
	InstanceOf []string
	// BirthDate/DeathDate are the P569/P570 dates at day precision,
	// converted to the Gregorian calendar, as UTC midnight. Nil when the
	// entity has no usable value (see selectClaimDate).
	BirthDate *time.Time
	DeathDate *time.Time
	// BirthDateUnusable/DeathDateUnusable report that P569/P570 statements
	// exist but none yielded a usable date (imprecise, conflicting, BCE,
	// unknown calendar, invalid, in the future, or a death before birth).
	BirthDateUnusable bool
	DeathDateUnusable bool
	// PhotoFile is the Commons filename of the entity's P18 (image), ""
	// when none.
	PhotoFile string
}

// wbGetEntitiesResponse mirrors the relevant subset of Wikidata's
// wbgetentities JSON response: {"entities": {"Q7200": {"id": "Q7200",
// "labels": {"ru": {"value": "..."}}, "claims": {"P31": [...]}}}} or, for a
// missing entity, {"entities": {"Q1": {"id": "Q1", "missing": ""}}}. API
// errors arrive as {"error": {"code": "no-such-entity", ...}} with HTTP 200.
type wbGetEntitiesResponse struct {
	Entities map[string]wbEntity `json:"entities"`
	Error    *struct {
		Code string `json:"code"`
		Info string `json:"info"`
		// ID names the offending entity of a no-such-entity error (a
		// batch request fails as a whole when one ID does not exist).
		ID string `json:"id"`
	} `json:"error"`
}

type wbEntity struct {
	ID      string                     `json:"id"`
	Missing *string                    `json:"missing"`
	Labels  map[string]wbLabel         `json:"labels"`
	Claims  map[string][]wbEntityClaim `json:"claims"`
}

type wbLabel struct {
	Value string `json:"value"`
}

type wbEntityClaim struct {
	// Rank is preferred | normal | deprecated.
	Rank     string `json:"rank"`
	Mainsnak struct {
		SnakType  string `json:"snaktype"`
		DataValue *struct {
			// Value is kept raw: P31 values are wikibase-entityid objects,
			// but a malformed/unexpected shape must not fail the whole
			// fetch.
			Value json.RawMessage `json:"value"`
		} `json:"datavalue"`
	} `json:"mainsnak"`
}

type wbEntityIDValue struct {
	ID string `json:"id"`
}

// FetchAuthor calls Wikidata's wbgetentities action API
// (action=wbgetentities&ids=<qid>&props=labels|claims&languages=ru) on
// cfg.searchBaseURL - the same JSON action API host as SearchAuthors - via
// cfg.doRequest, so it shares the client's User-Agent, rate limiter and
// retries.
//
// qid is trimmed and must be a bare QID (^Q[0-9]+$), otherwise
// ErrInvalidQuery is returned without an HTTP call. A missing entity (the
// "missing" key, or a "no-such-entity" API error) returns ErrNotFound. HTTP
// 400 returns ErrInvalidQuery; other >= 400 statuses map to
// ErrSourceUnavailable (429/5xx are handled by doRequest), as in
// SearchAuthors. The body is read through an io.LimitReader.
func (c *WikidataClient) FetchAuthor(ctx context.Context, qid string) (*ExternalAuthorRecord, error) {
	qid = strings.TrimSpace(qid)
	if !wikidataQIDPattern.MatchString(qid) {
		return nil, ErrInvalidQuery
	}

	entity, err := c.getEntity(ctx, qid)
	if err != nil {
		return nil, err
	}
	record := parseAuthorEntity(*entity, time.Now())
	if record == nil {
		return nil, ErrNotFound
	}
	return record, nil
}

// getEntity fetches one (already validated) entity with getEntities and
// picks it out of the response: a missing entity or a "no-such-entity"
// error → ErrNotFound, another API error → ErrInvalidQuery. Shared by
// FetchAuthor and FetchGenre.
func (c *WikidataClient) getEntity(ctx context.Context, qid string) (*wbEntity, error) {
	parsed, err := c.getEntities(ctx, []string{qid})
	if err != nil {
		return nil, err
	}

	if parsed.Error != nil {
		if parsed.Error.Code == "no-such-entity" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("%w: wikidata error %q", ErrInvalidQuery, parsed.Error.Code)
	}

	// The entities map is keyed by the requested ID even when Wikidata
	// resolved a redirect (the entity's own "id" is then the target).
	entity, ok := parsed.Entities[qid]
	if !ok {
		if len(parsed.Entities) != 1 {
			return nil, ErrNotFound
		}
		for _, e := range parsed.Entities {
			entity = e
		}
	}
	return &entity, nil
}

// MaxEntitiesPerRequest is the wbgetentities / Commons imageinfo batch cap
// (the MediaWiki API's per-request limit for anonymous clients).
const MaxEntitiesPerRequest = 50

// FetchAuthors fetches up to MaxEntitiesPerRequest Wikidata entities in one
// wbgetentities request (ids joined by "|"), with the same parameters and
// parser as FetchAuthor. The result is keyed by the requested QID (a
// redirect's record carries the canonical QID). Missing entities are absent
// from the map, without an error.
//
// Every QID is trimmed and must match ^Q[0-9]+$; an empty, oversized or
// invalid list returns ErrInvalidQuery without an HTTP call. Wikidata
// rejects a whole batch with a no-such-entity error naming one ID; that ID
// is dropped and the rest is re-requested (each retry is one more
// rate-limited call). Status mapping matches FetchAuthor.
func (c *WikidataClient) FetchAuthors(ctx context.Context, qids []string) (map[string]*ExternalAuthorRecord, error) {
	if len(qids) == 0 || len(qids) > MaxEntitiesPerRequest {
		return nil, ErrInvalidQuery
	}
	remaining := make([]string, 0, len(qids))
	seen := make(map[string]struct{}, len(qids))
	for _, q := range qids {
		q = strings.TrimSpace(q)
		if !wikidataQIDPattern.MatchString(q) {
			return nil, ErrInvalidQuery
		}
		if _, dup := seen[q]; dup {
			continue
		}
		seen[q] = struct{}{}
		remaining = append(remaining, q)
	}

	out := make(map[string]*ExternalAuthorRecord, len(remaining))
	now := time.Now()
	for len(remaining) > 0 {
		parsed, err := c.getEntities(ctx, remaining)
		if err != nil {
			return nil, err
		}
		if parsed.Error != nil {
			if parsed.Error.Code != "no-such-entity" {
				return nil, fmt.Errorf("%w: wikidata error %q", ErrInvalidQuery, parsed.Error.Code)
			}
			idx := indexOf(remaining, parsed.Error.ID)
			if idx < 0 {
				// The culprit is unknown: nothing more can be resolved.
				return out, nil
			}
			remaining = append(remaining[:idx:idx], remaining[idx+1:]...)
			continue
		}
		for _, q := range remaining {
			entity, ok := parsed.Entities[q]
			if !ok {
				continue
			}
			if record := parseAuthorEntity(entity, now); record != nil {
				out[q] = record
			}
		}
		break
	}
	return out, nil
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return -1
}

// getEntities performs one wbgetentities request for ids (already
// validated) and decodes the response. HTTP 400 → ErrInvalidQuery, other
// >= 400 → ErrSourceUnavailable (429/5xx are handled by doRequest).
func (c *WikidataClient) getEntities(ctx context.Context, ids []string) (*wbGetEntitiesResponse, error) {
	u, err := url.Parse(c.cfg.searchBaseURL)
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid wikidata search base URL: %w", err)
	}
	q := u.Query()
	q.Set("action", "wbgetentities")
	q.Set("ids", strings.Join(ids, "|"))
	q.Set("props", "labels|claims")
	q.Set("languages", "ru")
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bookdata: build wikidata entity request: %w", err)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, wikidataEntityMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("bookdata: read wikidata entity response: %w", err)
	}

	var parsed wbGetEntitiesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("bookdata: parse wikidata entity response: %w", err)
	}
	return &parsed, nil
}

// parseAuthorEntity maps one entity to a record, or returns nil for a
// missing entity / invalid id. now bounds the "not in the future" date
// rule. Malformed claims are skipped, never an error.
func parseAuthorEntity(entity wbEntity, now time.Time) *ExternalAuthorRecord {
	if entity.Missing != nil || !wikidataQIDPattern.MatchString(entity.ID) {
		return nil
	}

	record := &ExternalAuthorRecord{QID: entity.ID}
	if label, ok := entity.Labels["ru"]; ok {
		record.Name = label.Value
	}
	record.InstanceOf = entityIDClaims(entity.Claims["P31"])

	record.BirthDate, record.BirthDateUnusable = selectClaimDate(entity.Claims["P569"], now)
	record.DeathDate, record.DeathDateUnusable = selectClaimDate(entity.Claims["P570"], now)
	if record.BirthDate != nil && record.DeathDate != nil && record.DeathDate.Before(*record.BirthDate) {
		record.DeathDate = nil
		record.DeathDateUnusable = true
	}
	record.PhotoFile = selectClaimFile(entity.Claims["P18"])
	return record
}

// entityIDClaims returns the valid QIDs of entity-valued claims (P31,
// P279, ...); novalue/somevalue snaks and malformed values are skipped.
// Returns nil when there are none.
func entityIDClaims(claims []wbEntityClaim) []string {
	var out []string
	for _, claim := range claims {
		if claim.Mainsnak.SnakType != "value" || claim.Mainsnak.DataValue == nil {
			continue
		}
		var v wbEntityIDValue
		if err := json.Unmarshal(claim.Mainsnak.DataValue.Value, &v); err != nil {
			continue
		}
		if wikidataQIDPattern.MatchString(v.ID) {
			out = append(out, v.ID)
		}
	}
	return out
}
