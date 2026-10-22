package domain

import "testing"

func sp(s string) *string { return &s }

func TestDisplayCoverURL(t *testing.T) {
	src := "https://covers.openlibrary.org/b/id/1-L.jpg"
	other := "https://covers.openlibrary.org/b/id/2-L.jpg"
	cases := []struct {
		name   string
		cover  *string
		source *string
		path   *string
		status *string
		want   *string
	}{
		{"nil link", nil, nil, nil, nil, nil},
		{"empty link", sp(""), sp(""), sp("ab/x.jpg"), sp(CoverStatusStored), nil},
		{"stored match", sp(src), sp(src), sp("ab/x.jpg"), sp(CoverStatusStored), sp("/covers/ab/x.jpg")},
		{"stored mismatch", sp(other), sp(src), sp("ab/x.jpg"), sp(CoverStatusStored), sp(other)},
		{"stored without path", sp(src), sp(src), nil, sp(CoverStatusStored), sp(src)},
		{"absent match", sp(src), sp(src), nil, sp(CoverStatusAbsent), nil},
		{"absent mismatch", sp(other), sp(src), nil, sp(CoverStatusAbsent), sp(other)},
		{"pending", sp(src), sp(src), nil, sp(CoverStatusPending), sp(src)},
		{"failed", sp(src), sp(src), nil, sp(CoverStatusFailed), sp(src)},
		{"no copy", sp(src), nil, nil, nil, sp(src)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DisplayCoverURL(c.cover, c.source, c.path, c.status)
			if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
			b := Book{CoverURL: c.cover, CoverSourceURL: c.source, CoverPath: c.path, CoverStatus: c.status}
			m := b.DisplayCoverURL()
			if (m == nil) != (c.want == nil) || (m != nil && *m != *c.want) {
				t.Fatalf("method got %v want %v", m, c.want)
			}
		})
	}
}

func TestHasStoredCover(t *testing.T) {
	src := "u"
	b := Book{CoverURL: &src, CoverSourceURL: &src, CoverPath: sp("ab/x.jpg"), CoverStatus: sp(CoverStatusStored)}
	if !b.HasStoredCover() {
		t.Fatal("want stored")
	}
	b.CoverURL = sp("v")
	if b.HasStoredCover() {
		t.Fatal("mismatch must not count")
	}
}
