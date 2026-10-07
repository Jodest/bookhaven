package bookdata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// commonsFixture follows the live formatversion=2 imageinfo shape observed
// during the manual verification (thumb.wikimedia.org thumbs with utm_*
// query parameters, extmetadata values as {"value": ...}).
const commonsFixture = `{
  "batchcomplete": true,
  "query": {
    "normalized": [
      {"fromencoded": false, "from": "File:L.N.Tolstoy_Prokudin-Gorsky.jpg", "to": "File:L.N.Tolstoy Prokudin-Gorsky.jpg"},
      {"fromencoded": false, "from": "File:Missing_file.jpg", "to": "File:Missing file.jpg"}
    ],
    "pages": [
      {"ns": 6, "title": "File:Missing file.jpg", "missing": true, "imagerepository": ""},
      {"pageid": 2324270, "ns": 6, "title": "File:L.N.Tolstoy Prokudin-Gorsky.jpg", "imageinfo": [{
        "thumburl": "https://thumb.wikimedia.org/wikipedia/commons/thumb/c/c6/L.N.Tolstoy_Prokudin-Gorsky.jpg/500px-L.N.Tolstoy_Prokudin-Gorsky.jpg?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=thumbnail",
        "url": "https://upload.wikimedia.org/wikipedia/commons/c/c6/L.N.Tolstoy_Prokudin-Gorsky.jpg",
        "descriptionurl": "https://commons.wikimedia.org/wiki/File:L.N.Tolstoy_Prokudin-Gorsky.jpg",
        "extmetadata": {
          "Artist": {"value": "<bdi><a href=\"https://en.wikipedia.org/wiki/en:Sergey_Prokudin-Gorsky\"><span title=\"x\">Sergei Prokudin-Gorskii</span></a></bdi>", "source": "commons-desc-page"},
          "LicenseShortName": {"value": "Public domain", "source": "commons-desc-page", "hidden": ""},
          "License": {"value": "pd", "source": "commons-templates", "hidden": ""}
        }
      }]},
      {"pageid": 12887200, "ns": 6, "title": "File:Montbolo 001.jpg", "imageinfo": [{
        "thumburl": "https://upload.wikimedia.org/wikipedia/commons/thumb/3/36/Montbolo_001.jpg/500px-Montbolo_001.jpg",
        "descriptionurl": "https://commons.wikimedia.org/wiki/File:Montbolo_001.jpg",
        "extmetadata": {
          "Artist": {"value": "<a href=\"//commons.wikimedia.org/wiki/User:Grondin\">Grondin</a>"},
          "LicenseShortName": {"value": "CC BY-SA 3.0"},
          "LicenseUrl": {"value": "https://creativecommons.org/licenses/by-sa/3.0"},
          "License": {"value": "cc-by-sa-3.0"}
        }
      }]},
      {"pageid": 3, "ns": 6, "title": "File:Evil.jpg", "imageinfo": [{
        "thumburl": "https://evil.example/wikipedia/commons/x.jpg",
        "descriptionurl": "https://commons.wikimedia.org/wiki/File:Evil.jpg",
        "extmetadata": {"License": {"value": "cc0"}, "LicenseShortName": {"value": "CC0"}}
      }]}
    ]
  }
}`

func TestFetchCommonsImages_ParsesAndMapsNormalizedTitles(t *testing.T) {
	var gotQuery map[string]string
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotQuery = map[string]string{}
		for k := range q {
			gotQuery[k] = q.Get(k)
		}
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(commonsFixture))
	}))
	defer srv.Close()
	client := NewWikidataClient(
		withLimiter(&fakeLimiter{}), withSleep(func(time.Duration) {}), WithMaxRetries(0),
		WithUserAgent("TestAgent/1.0"), WithCommonsBaseURL(srv.URL),
	)

	got, err := client.FetchCommonsImages(context.Background(), []string{
		"L.N.Tolstoy_Prokudin-Gorsky.jpg", "Montbolo 001.jpg", "Missing_file.jpg", "Evil.jpg",
	})
	if err != nil {
		t.Fatalf("FetchCommonsImages: %v", err)
	}
	if gotUA != "TestAgent/1.0" {
		t.Errorf("User-Agent = %q", gotUA)
	}
	want := map[string]string{
		"action":              "query",
		"prop":                "imageinfo",
		"iiprop":              "url|extmetadata",
		"iiurlwidth":          "500",
		"iiextmetadatafilter": "Artist|LicenseShortName|License|LicenseUrl|NonFree",
		"titles":              "File:L.N.Tolstoy_Prokudin-Gorsky.jpg|File:Montbolo 001.jpg|File:Missing_file.jpg|File:Evil.jpg",
		"format":              "json",
		"formatversion":       "2",
	}
	for k, v := range want {
		if gotQuery[k] != v {
			t.Errorf("%s = %q, want %q", k, gotQuery[k], v)
		}
	}

	if _, ok := got["Missing_file.jpg"]; ok {
		t.Error("missing file must be absent")
	}
	tolstoy := got["L.N.Tolstoy_Prokudin-Gorsky.jpg"]
	if tolstoy == nil {
		t.Fatalf("normalized title not mapped back: %v", got)
	}
	if tolstoy.ThumbURL != "https://thumb.wikimedia.org/wikipedia/commons/thumb/c/c6/L.N.Tolstoy_Prokudin-Gorsky.jpg/500px-L.N.Tolstoy_Prokudin-Gorsky.jpg" {
		t.Errorf("ThumbURL = %q (query string must be dropped)", tolstoy.ThumbURL)
	}
	if tolstoy.PageURL != "https://commons.wikimedia.org/wiki/File:L.N.Tolstoy_Prokudin-Gorsky.jpg" ||
		tolstoy.Author != "Sergei Prokudin-Gorskii" || tolstoy.License != "Public domain" ||
		tolstoy.LicenseURL != "" || !tolstoy.Free || tolstoy.AttributionRequired {
		t.Errorf("tolstoy = %+v", tolstoy)
	}
	montbolo := got["Montbolo 001.jpg"]
	if montbolo == nil || !montbolo.Free || !montbolo.AttributionRequired || montbolo.Author != "Grondin" ||
		montbolo.LicenseURL != "https://creativecommons.org/licenses/by-sa/3.0" || montbolo.License != "CC BY-SA 3.0" {
		t.Errorf("montbolo = %+v", montbolo)
	}
	if evil := got["Evil.jpg"]; evil == nil || evil.ThumbURL != "" {
		t.Errorf("non-Wikimedia thumb must be rejected, got %+v", evil)
	}
}

func TestFetchCommonsImages_Errors(t *testing.T) {
	noCall := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("no HTTP call expected")
		return nil, nil
	}))
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = "a.jpg"
	}
	for name, files := range map[string][]string{"empty": nil, "too many": tooMany, "pipe": {"a|b.jpg"}, "blank": {" "}} {
		if _, err := noCall.FetchCommonsImages(context.Background(), files); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%s: err = %v, want ErrInvalidQuery", name, err)
		}
	}

	for status, want := range map[int]error{
		http.StatusBadRequest:          ErrInvalidQuery,
		http.StatusForbidden:           ErrSourceUnavailable,
		http.StatusInternalServerError: ErrSourceUnavailable,
	} {
		client := newWikidataTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return newTestResponse(status, "", nil), nil
		}))
		if _, err := client.FetchCommonsImages(context.Background(), []string{"a.jpg"}); !errors.Is(err, want) {
			t.Errorf("status %d: err = %v, want %v", status, err, want)
		}
	}
}

func TestClassifyCommonsLicense(t *testing.T) {
	cases := []struct {
		name, code, short, nonFree string
		free, attribution          bool
	}{
		{"pd", "pd", "Public domain", "", true, false},
		{"pd-old-70", "pd-old-70", "Public domain", "", true, false},
		{"cc0", "cc0", "CC0", "", true, false},
		{"cc by 4.0", "cc-by-4.0", "CC BY 4.0", "", true, true},
		{"cc by-sa 3.0 de", "cc-by-sa-3.0-de", "CC BY-SA 3.0 de", "", true, true},
		{"cc by-nc", "cc-by-nc-4.0", "CC BY-NC 4.0", "", false, false},
		{"cc by-nd", "cc-by-nd-2.0", "CC BY-ND 2.0", "", false, false},
		{"cc by-nc-sa", "cc-by-nc-sa-2.0", "CC BY-NC-SA 2.0", "", false, false},
		{"gfdl", "gfdl", "GFDL", "", false, false},
		{"nonfree flag", "cc-by-4.0", "CC BY 4.0", "true", false, false},
		{"missing", "", "", "", false, false},
		{"short name fallback pd", "", "Public domain", "", true, false},
		{"short name fallback cc by-sa", "", "CC BY-SA 4.0", "", true, true},
		{"short name fallback nc", "", "CC BY-NC 4.0", "", false, false},
		{"unknown", "attribution", "Attribution", "", false, false},
	}
	for _, c := range cases {
		free, attr := classifyCommonsLicense(c.code, c.short, c.nonFree)
		if free != c.free || attr != c.attribution {
			t.Errorf("%s: got free=%v attribution=%v, want %v %v", c.name, free, attr, c.free, c.attribution)
		}
	}
}

func TestCleanCommonsArtist(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"Plain Name": "Plain Name",
		`<a href="x">Ivan</a>   &amp;  <b>Pyotr</b>`: "Ivan & Pyotr",
		"Line<br/>Break":   "Line Break",
		"&lt;script&gt;":   "<script>",
		"<span>   </span>": "",
	}
	for in, want := range cases {
		if got := cleanCommonsArtist(in); got != want {
			t.Errorf("cleanCommonsArtist(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("я", 400)
	if got := cleanCommonsArtist(long); len([]rune(got)) != 300 {
		t.Errorf("cap: got %d runes, want 300", len([]rune(got)))
	}
}

func TestNormalizeCommonsThumbURL(t *testing.T) {
	cases := map[string]string{
		"https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg":        "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg",
		"https://thumb.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg?utm_a=b": "https://thumb.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg",
		"http://upload.wikimedia.org/wikipedia/commons/x.jpg":                                "",
		"https://upload.wikimedia.org/wikipedia/en/x.jpg":                                    "",
		"https://example.com/wikipedia/commons/x.jpg":                                        "",
		"": "",
	}
	for in, want := range cases {
		if got := normalizeCommonsThumbURL(in); got != want {
			t.Errorf("normalizeCommonsThumbURL(%q) = %q, want %q", in, got, want)
		}
	}
}
