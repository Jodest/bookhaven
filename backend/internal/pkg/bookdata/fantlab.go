package bookdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultFantLabBaseURL is the public FantLab API.
	defaultFantLabBaseURL = "https://api.fantlab.ru"
	// fantLabCoverURLPrefix is where edition covers live on the site. The
	// cover is linked, never fetched by this package.
	fantLabCoverURLPrefix = "https://fantlab.ru/images/editions/big/"
	// fantLabMinInterval is the robots.txt Crawl-delay (1 s) plus a margin.
	fantLabMinInterval = 1200 * time.Millisecond
	// fantLabMaxRetries is small on purpose: a long harvest job must not sleep
	// through repeated backoffs.
	fantLabMaxRetries = 1
	// MaxFantLabRequestsPerCall caps the requests one FindSelfTitledEditionCover
	// call makes (work records plus edition records).
	MaxFantLabRequestsPerCall = 4
	// maxFantLabEditionsPerWork caps how many matching editions of one work are
	// checked for an image.
	maxFantLabEditionsPerWork = 3
)

// FantLab response shape (verified against the live API on 2026-10-29):
//
//	GET /work/{id}/extended
//	  work_name: string
//	  editions_blocks: object whose KEYS ARE NUMERIC STRINGS ("10"), not block
//	    names. Every value is {"block": "paper"|"audio"|"magazin"|"books_plan"|…,
//	    "list": [edition…]}. Only blocks whose "block" field is "paper" are used.
//	  edition entry: edition_id (int), name, lang_code ("ru"), year (int),
//	    pic_num (int, number of pictures), type, isbn, publisher, … There is no
//	    image field in list entries.
//	GET /edition/{id}
//	  image: "/images/editions/big/16469?r=1492544818", null or "" when the
//	    edition has no cover.
//
// The cover link is https://fantlab.ru/images/editions/big/{edition_id}. It
// answers 200 image/jpeg even with a foreign Referer.

// FantLabClient reads FantLab (api.fantlab.ru) to find a cover link for a
// Russian paper edition that is named exactly like the book. Construct with
// NewFantLabClient.
type FantLabClient struct {
	cfg *config
	bud *requestBudget
}

// NewFantLabClient builds a FantLabClient. Defaults: public endpoint,
// DefaultUserAgent, one request per 1.2 s (FantLab Crawl-delay), one retry,
// unlimited request budget. opts are applied after the defaults.
func NewFantLabClient(opts ...Option) *FantLabClient {
	defaults := func(cfg *config) {
		cfg.limiter = rate.NewLimiter(rate.Every(fantLabMinInterval), 1)
		cfg.maxRetries = fantLabMaxRetries
	}
	cfg := newConfig(append([]Option{defaults}, opts...))
	if cfg.baseURL == "" {
		cfg.baseURL = defaultFantLabBaseURL
	}
	bud := &requestBudget{remaining: -1}
	cfg.limiter = &budgetLimiter{inner: cfg.limiter, bud: bud}
	return &FantLabClient{cfg: cfg, bud: bud}
}

// SetRequestBudget limits the HTTP attempts (retries included) this client may
// still make. A negative n removes the limit. When the budget is spent every
// call fails with ErrBudgetExhausted without a request.
func (c *FantLabClient) SetRequestBudget(n int) {
	c.bud.mu.Lock()
	c.bud.remaining = n
	c.bud.mu.Unlock()
}

type requestBudget struct {
	mu        sync.Mutex
	remaining int // < 0 means unlimited
}

func (b *requestBudget) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining < 0 {
		return true
	}
	if b.remaining == 0 {
		return false
	}
	b.remaining--
	return true
}

// budgetLimiter spends one budget unit per outgoing attempt (doRequest waits
// on the limiter once per attempt) before the real limiter runs.
type budgetLimiter struct {
	inner limiter
	bud   *requestBudget
}

func (l *budgetLimiter) Wait(ctx context.Context) error {
	if !l.bud.take() {
		return ErrBudgetExhausted
	}
	return l.inner.Wait(ctx)
}

// flexInt decodes a JSON number or numeric string; anything else is 0.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	n, err := strconv.Atoi(s)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexInt(n)
	return nil
}

type fantLabEdition struct {
	EditionID flexInt  `json:"edition_id"`
	Name      string   `json:"name"`
	LangCode  string   `json:"lang_code"`
	Year      flexInt  `json:"year"`
	PicNum    *flexInt `json:"pic_num"`
}

type fantLabBlock struct {
	Block string           `json:"block"`
	List  []fantLabEdition `json:"list"`
}

type fantLabWork struct {
	WorkName       string                  `json:"work_name"`
	EditionsBlocks map[string]fantLabBlock `json:"editions_blocks"`
}

// FindSelfTitledEditionCover looks for a Russian paper edition on the given
// FantLab works (Wikidata P7439, ordered and capped by OrderFantLabWorkIDs)
// whose normalized name equals the book title and whose work name does too,
// and that has a cover image. It returns the cover link and
// "<workID>/<editionID>". A collection published under another name, other
// languages and non-paper blocks never match. A miss or failure on one work
// moves on to the next. When nothing is found it returns ErrNotFound, or the
// last transport/source error seen, so a caller can tell "no cover" from
// "could not ask". ErrBudgetExhausted is returned as it is. One call makes at
// most MaxFantLabRequestsPerCall requests.
func (c *FantLabClient) FindSelfTitledEditionCover(ctx context.Context, title string, workIDs []string) (coverURL, usedID string, err error) {
	want := NormalizeMatchText(title)
	if want == "" {
		return "", "", ErrNotFound
	}
	requests := 0
	var lastErr error
	for _, workID := range OrderFantLabWorkIDs(workIDs) {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		if requests >= MaxFantLabRequestsPerCall {
			break
		}
		requests++
		var work fantLabWork
		if err := c.getJSON(ctx, "/work/"+strconv.Itoa(workID)+"/extended", &work); err != nil {
			if errors.Is(err, ErrBudgetExhausted) {
				return "", "", err
			}
			if !errors.Is(err, ErrNotFound) {
				lastErr = err
			}
			continue
		}
		if NormalizeMatchText(work.WorkName) != want {
			continue
		}
		for _, ed := range selfTitledEditions(work, want) {
			if ed.PicNum != nil && *ed.PicNum <= 0 {
				continue
			}
			if requests >= MaxFantLabRequestsPerCall {
				break
			}
			requests++
			has, err := c.editionHasImage(ctx, int(ed.EditionID))
			if err != nil {
				if errors.Is(err, ErrBudgetExhausted) {
					return "", "", err
				}
				if !errors.Is(err, ErrNotFound) {
					lastErr = err
				}
				continue
			}
			if has {
				id := strconv.Itoa(int(ed.EditionID))
				return fantLabCoverURLPrefix + id, strconv.Itoa(workID) + "/" + id, nil
			}
		}
	}
	if lastErr != nil {
		return "", "", lastErr
	}
	return "", "", ErrNotFound
}

// selfTitledEditions returns the Russian paper editions named exactly like the
// book (want is already normalized), newest first (unknown year last, then
// ascending edition ID), at most maxFantLabEditionsPerWork.
func selfTitledEditions(work fantLabWork, want string) []fantLabEdition {
	var out []fantLabEdition
	for _, blk := range work.EditionsBlocks {
		if blk.Block != "paper" {
			continue
		}
		for _, ed := range blk.List {
			if ed.EditionID <= 0 || ed.LangCode != "ru" || NormalizeMatchText(ed.Name) != want {
				continue
			}
			out = append(out, ed)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		yi, yj := out[i].Year, out[j].Year
		if (yi > 0) != (yj > 0) {
			return yi > 0
		}
		if yi != yj {
			return yi > yj
		}
		return out[i].EditionID < out[j].EditionID
	})
	if len(out) > maxFantLabEditionsPerWork {
		out = out[:maxFantLabEditionsPerWork]
	}
	return out
}

// editionHasImage reads /edition/{id} and reports whether it has a cover image.
func (c *FantLabClient) editionHasImage(ctx context.Context, editionID int) (bool, error) {
	var parsed struct {
		Image *string `json:"image"`
	}
	if err := c.getJSON(ctx, "/edition/"+strconv.Itoa(editionID), &parsed); err != nil {
		return false, err
	}
	return parsed.Image != nil && strings.TrimSpace(*parsed.Image) != "", nil
}

func (c *FantLabClient) getJSON(ctx context.Context, path string, dst any) error {
	u, err := url.Parse(c.cfg.baseURL + path)
	if err != nil {
		return fmt.Errorf("bookdata: invalid fantlab base URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("bookdata: build fantlab request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: fantlab returned status %d", ErrSourceUnavailable, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("bookdata: read fantlab response: %w", err)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("bookdata: parse fantlab response: %w", err)
	}
	return nil
}
