package coverfetch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

var jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{1}, 50)...)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func resp(r *http.Request, status int, body []byte, hdr map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status, Status: strconv.Itoa(status), Header: h, Request: r,
		Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)),
	}
}

func newFetcher(rt rtFunc) *Fetcher {
	return NewWithTransport(ParseHosts(DefaultAllowedHosts), rt)
}

func code(t *testing.T, err error) (string, bool) {
	t.Helper()
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatalf("not a fetch error: %v", err)
	}
	return fe.Code, fe.Transient
}

func TestFetch_SuccessAndOpenLibraryDefaultFalse(t *testing.T) {
	var seen string
	f := newFetcher(func(r *http.Request) (*http.Response, error) {
		seen = r.URL.String()
		return resp(r, 200, jpegBytes, map[string]string{"Content-Type": "text/html"}), nil
	})
	res, err := f.Fetch(context.Background(), "https://covers.openlibrary.org/b/id/1-L.jpg")
	if err != nil || res.Ext != "jpg" || res.Absent {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(seen, "default=false") {
		t.Fatalf("default=false missing: %s", seen)
	}
	// An existing default is kept.
	_, _ = f.Fetch(context.Background(), "https://covers.openlibrary.org/b/id/1-L.jpg?default=true")
	if !strings.Contains(seen, "default=true") || strings.Contains(seen, "default=false") {
		t.Fatalf("existing default changed: %s", seen)
	}
	// Other hosts are not touched.
	_, _ = f.Fetch(context.Background(), "https://upload.wikimedia.org/a.jpg")
	if strings.Contains(seen, "default") {
		t.Fatalf("default added to non-OL host: %s", seen)
	}
}

func TestFetch_MagicBytes(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, 1, 2, 3)
	webp := append([]byte("RIFF\x10\x00\x00\x00WEBP"), 1, 2)
	for want, data := range map[string][]byte{"png": png, "webp": webp} {
		f := newFetcher(func(r *http.Request) (*http.Response, error) { return resp(r, 200, data, nil), nil })
		res, err := f.Fetch(context.Background(), "https://archive.org/x")
		if err != nil || res.Ext != want {
			t.Fatalf("%s: %+v %v", want, res, err)
		}
	}
	for name, data := range map[string][]byte{
		"gif":  []byte("GIF89a...."),
		"html": []byte("<html>not an image</html>"),
		"tiny": {0xFF},
	} {
		f := newFetcher(func(r *http.Request) (*http.Response, error) {
			return resp(r, 200, data, map[string]string{"Content-Type": "image/jpeg"}), nil
		})
		_, err := f.Fetch(context.Background(), "https://archive.org/x")
		if c, tr := code(t, err); c != CodeNotImage || tr {
			t.Fatalf("%s: %s %v", name, c, tr)
		}
	}
}

func TestFetch_SizeLimit(t *testing.T) {
	big := append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, MaxBytes-3)...)
	f := newFetcher(func(r *http.Request) (*http.Response, error) {
		x := resp(r, 200, big, nil)
		x.ContentLength = -1
		return x, nil
	})
	if res, err := f.Fetch(context.Background(), "https://archive.org/x"); err != nil || len(res.Data) != MaxBytes {
		t.Fatalf("exactly 5 MiB must pass: %v", err)
	}
	over := append(big, 0)
	f = newFetcher(func(r *http.Request) (*http.Response, error) {
		x := resp(r, 200, over, nil)
		x.ContentLength = -1
		return x, nil
	})
	_, err := f.Fetch(context.Background(), "https://archive.org/x")
	if c, tr := code(t, err); c != CodeTooLarge || tr {
		t.Fatalf("%s %v", c, tr)
	}
	// Declared length alone is enough to refuse.
	f = newFetcher(func(r *http.Request) (*http.Response, error) {
		x := resp(r, 200, jpegBytes, nil)
		x.ContentLength = MaxBytes + 1
		return x, nil
	})
	_, err = f.Fetch(context.Background(), "https://archive.org/x")
	if c, _ := code(t, err); c != CodeTooLarge {
		t.Fatalf("content-length: %s", c)
	}
}

func TestFetch_StatusClassification(t *testing.T) {
	cases := []struct {
		url       string
		status    int
		wantCode  string
		transient bool
		absent    bool
	}{
		{"https://covers.openlibrary.org/b/id/1-L.jpg", 404, "", false, true},
		{"https://archive.org/x", 404, CodeHTTPError, false, false},
		{"https://archive.org/x", 503, CodeHTTPError, true, false},
		{"https://archive.org/x", 429, CodeHTTPError, true, false},
		{"https://archive.org/x", 408, CodeHTTPError, true, false},
		{"https://archive.org/x", 403, CodeHTTPError, false, false},
	}
	for _, c := range cases {
		f := newFetcher(func(r *http.Request) (*http.Response, error) { return resp(r, c.status, nil, nil), nil })
		res, err := f.Fetch(context.Background(), c.url)
		if c.absent {
			if err != nil || !res.Absent {
				t.Fatalf("%s %d: want absent, got %+v %v", c.url, c.status, res, err)
			}
			continue
		}
		if code2, tr := code(t, err); code2 != c.wantCode || tr != c.transient {
			t.Fatalf("%s %d: %s %v", c.url, c.status, code2, tr)
		}
	}
}

func TestFetch_NetworkAndTimeout(t *testing.T) {
	f := newFetcher(func(r *http.Request) (*http.Response, error) { return nil, errors.New("connection reset") })
	_, err := f.Fetch(context.Background(), "https://archive.org/x")
	if c, tr := code(t, err); c != CodeNetwork || !tr {
		t.Fatalf("%s %v", c, tr)
	}
	f = newFetcher(func(r *http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
	_, err = f.Fetch(context.Background(), "https://archive.org/x")
	if c, tr := code(t, err); c != CodeTimeout || !tr {
		t.Fatalf("%s %v", c, tr)
	}
	f = newFetcher(func(r *http.Request) (*http.Response, error) { return nil, ErrBlockedAddress })
	_, err = f.Fetch(context.Background(), "https://archive.org/x")
	if c, tr := code(t, err); c != CodeBlockedAddress || tr {
		t.Fatalf("%s %v", c, tr)
	}
}

func TestFetch_URLRules(t *testing.T) {
	f := newFetcher(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("must not request %s", r.URL)
		return nil, nil
	})
	for _, u := range []string{
		"ftp://archive.org/x", "file:///etc/passwd", "https://evil.example/x", "https://archive.org:8443/x",
		"http://127.0.0.1/x", "https://10.0.0.5/x", "https://archive.org.evil.com/x", "https://user:pw@archive.org/x",
		"https://archive.org@evil.com/x", "", "://bad", "https://notarchive.org/x",
	} {
		_, err := f.Fetch(context.Background(), u)
		if c, tr := code(t, err); c != CodeHostNotAllowed || tr {
			t.Errorf("%q: %s %v", u, c, tr)
		}
	}
	// Allowed forms reach the transport.
	reached := 0
	f = newFetcher(func(r *http.Request) (*http.Response, error) { reached++; return resp(r, 200, jpegBytes, nil), nil })
	for _, u := range []string{
		"http://archive.org/x", "https://ia800000.us.archive.org/x", "https://archive.org:443/x",
		"https://UPLOAD.wikimedia.org/x", "https://covers.openlibrary.org:80/x",
	} {
		if _, err := f.Fetch(context.Background(), u); err != nil {
			t.Errorf("%q: %v", u, err)
		}
	}
	if reached != 5 {
		t.Fatalf("reached %d", reached)
	}
}

func redirectRT(target func(n int) string) rtFunc {
	n := 0
	return func(r *http.Request) (*http.Response, error) {
		n++
		next := target(n)
		if next == "" {
			return resp(r, 200, jpegBytes, nil), nil
		}
		return resp(r, 302, nil, map[string]string{"Location": next}), nil
	}
}

func TestFetch_Redirects(t *testing.T) {
	// OL -> archive.org -> subdomain: two hops are followed.
	f := newFetcher(redirectRT(func(n int) string {
		switch n {
		case 1:
			return "https://archive.org/download/a.jpg"
		case 2:
			return "https://ia800.us.archive.org/a.jpg"
		}
		return ""
	}))
	if _, err := f.Fetch(context.Background(), "https://covers.openlibrary.org/b/id/1-L.jpg"); err != nil {
		t.Fatalf("2 redirects: %v", err)
	}
	// Exactly 5 redirects pass, 6 fail.
	f = newFetcher(redirectRT(func(n int) string {
		if n <= MaxRedirects {
			return "https://archive.org/r" + strconv.Itoa(n)
		}
		return ""
	}))
	if _, err := f.Fetch(context.Background(), "https://archive.org/start"); err != nil {
		t.Fatalf("5 redirects: %v", err)
	}
	f = newFetcher(redirectRT(func(n int) string {
		if n <= MaxRedirects+1 {
			return "https://archive.org/r" + strconv.Itoa(n)
		}
		return ""
	}))
	_, err := f.Fetch(context.Background(), "https://archive.org/start")
	if c, tr := code(t, err); c != CodeTooManyRedirects || tr {
		t.Fatalf("6 redirects: %s %v", c, tr)
	}
	// Redirects to internal or foreign targets are refused before any request.
	for _, target := range []string{
		"http://10.0.0.5/a.jpg", "http://127.0.0.1/a.jpg", "http://[::1]/a.jpg", "http://169.254.169.254/latest/meta-data",
		"http://[::ffff:10.0.0.5]/a.jpg", "https://evil.example/a.jpg", "https://archive.org:8080/a.jpg", "ftp://archive.org/a.jpg",
	} {
		calls := 0
		f = newFetcher(func(r *http.Request) (*http.Response, error) {
			calls++
			return resp(r, 302, nil, map[string]string{"Location": target}), nil
		})
		_, err := f.Fetch(context.Background(), "https://archive.org/start")
		if c, tr := code(t, err); c != CodeHostNotAllowed || tr {
			t.Errorf("%s: %s %v", target, c, tr)
		}
		if calls != 1 {
			t.Errorf("%s: %d requests", target, calls)
		}
	}
}

func TestIsPublicAddr(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.5", "172.16.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0", "0.1.2.3",
		"100.64.0.1", "224.0.0.1", "240.0.0.1", "::1", "::", "fe80::1", "fc00::1", "fd12::1", "ff02::1",
		"::ffff:10.0.0.5", "::ffff:127.0.0.1", "::ffff:169.254.169.254", "64:ff9b::7f00:1", "198.18.0.1",
	}
	for _, s := range blocked {
		if IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "151.101.1.1", "207.241.224.2", "2606:4700::1111", "::ffff:8.8.8.8"} {
		if !IsPublicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
}

func TestCheckDialAddress(t *testing.T) {
	for _, a := range []string{"10.0.0.5:443", "127.0.0.1:80", "[::1]:443", "8.8.8.8:8080", "8.8.8.8:22", "garbage", "[::ffff:10.0.0.5]:443"} {
		if !errors.Is(checkDialAddress(a), ErrBlockedAddress) {
			t.Errorf("%s must be blocked", a)
		}
	}
	for _, a := range []string{"8.8.8.8:443", "8.8.8.8:80", "[2606:4700::1111]:443"} {
		if err := checkDialAddress(a); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
}

func TestMatchHost(t *testing.T) {
	al := ParseHosts(DefaultAllowedHosts)
	yes := []string{"covers.openlibrary.org", "archive.org", "ia1.us.archive.org", "Upload.Wikimedia.org", "archive.org."}
	no := []string{"", "openlibrary.org", "evil.org", "archive.org.evil.com", "evilarchive.org", "wikimedia.org", "x.upload.wikimedia.org"}
	for _, h := range yes {
		if !MatchHost(al, h) {
			t.Errorf("%q must match", h)
		}
	}
	for _, h := range no {
		if MatchHost(al, h) {
			t.Errorf("%q must not match", h)
		}
	}
	if MatchHost([]string{"*.example.com"}, "example.com") {
		t.Error("wildcard must not match the bare domain")
	}
}
