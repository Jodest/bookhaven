package bookdata

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// defaultCommonsBaseURL is the Wikimedia Commons action API endpoint.
const defaultCommonsBaseURL = "https://commons.wikimedia.org/w/api.php"

// commonsThumbWidth is the requested thumbnail width: a standard Wikimedia
// thumbnail step (non-standard widths may be throttled). The author page
// shows the photo at 200 px, so 500 covers 2x displays.
const commonsThumbWidth = 500

// commonsMaxBytes bounds the imageinfo response body.
const commonsMaxBytes = 4 << 20 // 4 MiB

// Attribution field caps (runes).
const (
	commonsAuthorMaxRunes  = 300
	commonsLicenseMaxRunes = 100
)

// commonsThumbHosts are the accepted thumbnail hosts. Commons returned
// thumb.wikimedia.org thumbnails in the live check (October 2026); the
// same path is also served by upload.wikimedia.org.
var commonsThumbHosts = map[string]struct{}{
	"upload.wikimedia.org": {},
	"thumb.wikimedia.org":  {},
}

// CommonsImage is one resolved Wikimedia Commons file for an author photo
// (docs/features/author-wikidata-enrichment.md rule 3).
type CommonsImage struct {
	// ThumbURL is the https thumbnail URL on a Wikimedia upload host,
	// without the tracking query string; "" when unusable.
	ThumbURL string
	// PageURL is the commons.wikimedia.org/wiki/File: description page;
	// "" when missing.
	PageURL string
	// Author is the cleaned Artist text (HTML stripped, unescaped,
	// whitespace collapsed, capped), "" when unknown.
	Author string
	// License is LicenseShortName (capped); LicenseURL is LicenseUrl when
	// it is an http(s) URL, else "".
	License    string
	LicenseURL string
	// Free reports an allowlisted free licence (PD, CC0, CC BY, CC BY-SA)
	// that is not marked NonFree.
	Free bool
	// AttributionRequired reports a CC BY / CC BY-SA licence (an author is
	// then mandatory for using the photo).
	AttributionRequired bool
}

type commonsQueryResponse struct {
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
	Query struct {
		Normalized []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"normalized"`
		Pages []commonsPage `json:"pages"`
	} `json:"query"`
}

type commonsPage struct {
	Title     string `json:"title"`
	Missing   bool   `json:"missing"`
	Invalid   bool   `json:"invalid"`
	ImageInfo []struct {
		ThumbURL       string                         `json:"thumburl"`
		DescriptionURL string                         `json:"descriptionurl"`
		ExtMetadata    map[string]commonsMetadataItem `json:"extmetadata"`
	} `json:"imageinfo"`
}

type commonsMetadataItem struct {
	// Value is usually a string; kept raw so an unexpected shape is
	// ignored instead of failing the whole response.
	Value json.RawMessage `json:"value"`
}

func (m commonsMetadataItem) str() string {
	var s string
	if err := json.Unmarshal(m.Value, &s); err != nil {
		return ""
	}
	return s
}

// FetchCommonsImages resolves up to MaxEntitiesPerRequest Commons
// filenames (bare P18 values, without the "File:" prefix) in one
// action=query&prop=imageinfo request on cfg.commonsBaseURL via
// cfg.doRequest. The result is keyed by the input filename (Commons title
// normalization is mapped back); missing files are absent. An empty or
// oversized list, or a name containing "|", returns ErrInvalidQuery without
// an HTTP call. HTTP 400 → ErrInvalidQuery, other >= 400 →
// ErrSourceUnavailable (429/5xx via doRequest).
func (c *WikidataClient) FetchCommonsImages(ctx context.Context, filenames []string) (map[string]*CommonsImage, error) {
	if len(filenames) == 0 || len(filenames) > MaxEntitiesPerRequest {
		return nil, ErrInvalidQuery
	}
	titles := make([]string, 0, len(filenames))
	titleOf := make(map[string]string, len(filenames))
	for _, name := range filenames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || strings.Contains(trimmed, "|") {
			return nil, ErrInvalidQuery
		}
		title := "File:" + trimmed
		if _, dup := titleOf[name]; !dup {
			titles = append(titles, title)
		}
		titleOf[name] = title
	}

	u, err := url.Parse(c.cfg.commonsBaseURL)
	if err != nil {
		return nil, fmt.Errorf("bookdata: invalid commons base URL: %w", err)
	}
	q := u.Query()
	q.Set("action", "query")
	q.Set("prop", "imageinfo")
	q.Set("iiprop", "url|extmetadata")
	q.Set("iiurlwidth", strconv.Itoa(commonsThumbWidth))
	q.Set("iiextmetadatafilter", "Artist|LicenseShortName|License|LicenseUrl|NonFree")
	q.Set("titles", strings.Join(titles, "|"))
	q.Set("format", "json")
	q.Set("formatversion", "2")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bookdata: build commons request: %w", err)
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
		return nil, fmt.Errorf("%w: commons returned status %d", ErrSourceUnavailable, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, commonsMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("bookdata: read commons response: %w", err)
	}
	var parsed commonsQueryResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("bookdata: parse commons response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("%w: commons error %q", ErrInvalidQuery, parsed.Error.Code)
	}

	normalized := make(map[string]string, len(parsed.Query.Normalized))
	for _, n := range parsed.Query.Normalized {
		normalized[n.From] = n.To
	}
	pages := make(map[string]commonsPage, len(parsed.Query.Pages))
	for _, p := range parsed.Query.Pages {
		pages[p.Title] = p
	}

	out := make(map[string]*CommonsImage, len(titleOf))
	for name, title := range titleOf {
		if to, ok := normalized[title]; ok {
			title = to
		}
		page, ok := pages[title]
		if !ok || page.Missing || page.Invalid || len(page.ImageInfo) == 0 {
			continue
		}
		out[name] = buildCommonsImage(page)
	}
	return out, nil
}

func buildCommonsImage(page commonsPage) *CommonsImage {
	info := page.ImageInfo[0]
	meta := func(key string) string {
		if item, ok := info.ExtMetadata[key]; ok {
			return item.str()
		}
		return ""
	}
	img := &CommonsImage{
		ThumbURL:   normalizeCommonsThumbURL(info.ThumbURL),
		PageURL:    normalizeCommonsPageURL(info.DescriptionURL),
		Author:     cleanCommonsArtist(meta("Artist")),
		License:    capRunes(strings.TrimSpace(meta("LicenseShortName")), commonsLicenseMaxRunes),
		LicenseURL: httpURLOrEmpty(meta("LicenseUrl")),
	}
	img.Free, img.AttributionRequired = classifyCommonsLicense(meta("License"), meta("LicenseShortName"), meta("NonFree"))
	return img
}

// normalizeCommonsThumbURL accepts an https thumbnail on a Wikimedia
// upload host under /wikipedia/commons/ and drops its query string and
// fragment (Commons appends utm_* tracking parameters). Anything else
// returns "".
func normalizeCommonsThumbURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil {
		return ""
	}
	if _, ok := commonsThumbHosts[u.Host]; !ok {
		return ""
	}
	if !strings.HasPrefix(u.Path, "/wikipedia/commons/") {
		return ""
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

// normalizeCommonsPageURL accepts the https://commons.wikimedia.org/wiki/File:
// description page URL, else "".
func normalizeCommonsPageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "https://commons.wikimedia.org/wiki/File:") {
		return ""
	}
	return raw
}

var (
	htmlTagPattern     = regexp.MustCompile(`<[^>]*>`)
	ccLicenseCode      = regexp.MustCompile(`^cc-by(-sa)?-[0-9]`)
	ccLicenseShortName = regexp.MustCompile(`^cc by(-sa)? [0-9]`)
)

// cleanCommonsArtist strips HTML tags, unescapes entities, collapses
// whitespace and caps the result at commonsAuthorMaxRunes.
func cleanCommonsArtist(raw string) string {
	text := htmlTagPattern.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(text), " ")
	return capRunes(text, commonsAuthorMaxRunes)
}

// classifyCommonsLicense applies the free-licence allowlist: public
// domain, CC0, CC BY and CC BY-SA (any version or port). The License code
// (lowercased) is used, falling back to LicenseShortName. NonFree=true, NC
// / ND variants, GFDL-only, unknown or missing licences are not free.
func classifyCommonsLicense(code, shortName, nonFree string) (free, attributionRequired bool) {
	if strings.EqualFold(strings.TrimSpace(nonFree), "true") {
		return false, false
	}
	code = strings.ToLower(strings.TrimSpace(code))
	if code != "" {
		if strings.Contains(code, "-nc") || strings.Contains(code, "-nd") {
			return false, false
		}
		switch {
		case code == "pd" || strings.HasPrefix(code, "pd-"):
			return true, false
		case code == "cc0" || strings.HasPrefix(code, "cc0-"):
			return true, false
		case ccLicenseCode.MatchString(code):
			return true, true
		}
		return false, false
	}

	name := strings.ToLower(strings.Join(strings.Fields(shortName), " "))
	if name == "" || strings.Contains(name, "-nc") || strings.Contains(name, "-nd") {
		return false, false
	}
	switch {
	case strings.HasPrefix(name, "public domain"):
		return true, false
	case name == "cc0" || strings.HasPrefix(name, "cc0 "):
		return true, false
	case ccLicenseShortName.MatchString(name):
		return true, true
	}
	return false, false
}

// httpURLOrEmpty returns raw (trimmed) when it is an absolute http(s) URL.
func httpURLOrEmpty(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
}

func capRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:max]))
}
