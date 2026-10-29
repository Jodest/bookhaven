package bookdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// FindCoverByOpenLibraryIDs looks up the work or edition records behind the
// given Open Library IDs (Wikidata P648) and returns the first positive cover
// ID of the first record that has one, as a large cover link, together with
// the ID that produced it. Only the cover is read: no description, ISBN or
// page count. The IDs are filtered, ordered and capped by OrderOpenLibraryIDs.
// A miss or failure on one ID moves on to the next. When none gives a cover it
// returns ErrNotFound, or the last transport/source error seen if there was
// one, so a caller can tell "no cover" from "could not ask".
func (c *OpenLibraryClient) FindCoverByOpenLibraryIDs(ctx context.Context, ids []string) (coverURL string, usedID string, err error) {
	var lastErr error
	for _, id := range OrderOpenLibraryIDs(ids) {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		coverID, err := c.fetchFirstCoverID(ctx, id)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				lastErr = err
			}
			continue
		}
		return CoverURL(strconv.FormatInt(coverID, 10), CoverIDTypeID, CoverSizeLarge), id, nil
	}
	if lastErr != nil {
		return "", "", lastErr
	}
	return "", "", ErrNotFound
}

// fetchFirstCoverID reads /works/{id}.json (W) or /books/{id}.json (M) and
// returns the first positive value of "covers", or ErrNotFound.
func (c *OpenLibraryClient) fetchFirstCoverID(ctx context.Context, id string) (int64, error) {
	section := "books"
	if id[len(id)-1] == 'W' {
		section = "works"
	}
	u, err := url.Parse(c.cfg.baseURL + "/" + section + "/" + url.PathEscape(id) + ".json")
	if err != nil {
		return 0, fmt.Errorf("bookdata: invalid open library base URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("bookdata: build open library request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.doRequest(ctx, req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, ErrNotFound
	}
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("%w: open library returned status %d", ErrSourceUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("bookdata: read open library response: %w", err)
	}
	var parsed struct {
		Covers []int64 `json:"covers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("bookdata: parse open library cover response: %w", err)
	}
	for _, v := range parsed.Covers {
		if v > 0 {
			return v, nil
		}
	}
	return 0, ErrNotFound
}
