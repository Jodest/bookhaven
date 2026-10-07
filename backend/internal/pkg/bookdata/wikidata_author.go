package bookdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// wikidataEntityMaxBytes bounds the wbgetentities response body read by
// FetchAuthor. Famous entities carry large claim sets (hundreds of KB), so
// the cap is generous; it only guards against a runaway response.
const wikidataEntityMaxBytes = 8 << 20 // 8 MiB

// ExternalAuthorRecord is a single Wikidata author entity fetched by
// FetchAuthor for a direct author import (docs/features/author-direct-import.md).
//
// Feature 4 of the "Author moderation & import" epic (enrichment) adds
// day-precision birth/death dates, a Commons photo with attribution, a
// biography and similar fields here; the same wbgetentities call already
// returns the claims those need.
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

	u, err := url.Parse(c.cfg.searchBaseURL)
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid wikidata search base URL: %w", err)
	}
	q := u.Query()
	q.Set("action", "wbgetentities")
	q.Set("ids", qid)
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
	if entity.Missing != nil || !wikidataQIDPattern.MatchString(entity.ID) {
		return nil, ErrNotFound
	}

	record := &ExternalAuthorRecord{QID: entity.ID}
	if label, ok := entity.Labels["ru"]; ok {
		record.Name = label.Value
	}
	for _, claim := range entity.Claims["P31"] {
		if claim.Mainsnak.SnakType != "value" || claim.Mainsnak.DataValue == nil {
			continue
		}
		var v wbEntityIDValue
		if err := json.Unmarshal(claim.Mainsnak.DataValue.Value, &v); err != nil {
			continue
		}
		if wikidataQIDPattern.MatchString(v.ID) {
			record.InstanceOf = append(record.InstanceOf, v.ID)
		}
	}
	return record, nil
}
