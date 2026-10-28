package domain

import "testing"

func strp(s string) *string { return &s }

func TestPosterColumns(t *testing.T) {
	cases := []struct {
		name    string
		c       PosterColumns
		wantURL string
		nilPost bool
		removed bool
	}{
		{name: "null", nilPost: true},
		{name: "removed", c: PosterColumns{PosterSource: strp(PosterSourceRemoved)}, nilPost: true, removed: true},
		{name: "manual", c: PosterColumns{PosterSource: strp(PosterSourceManual), PosterPath: strp("ab/abc.jpg")}, wantURL: "/posters/ab/abc.jpg"},
		{name: "commons", c: PosterColumns{PosterSource: strp(PosterSourceCommons), PosterURL: strp("https://upload.wikimedia.org/x.jpg")}, wantURL: "https://upload.wikimedia.org/x.jpg"},
		{name: "tmdb", c: PosterColumns{PosterSource: strp(PosterSourceTMDB), PosterURL: strp("https://image.tmdb.org/t/p/w342/a.jpg")}, wantURL: "https://image.tmdb.org/t/p/w342/a.jpg"},
		{name: "manual without path", c: PosterColumns{PosterSource: strp(PosterSourceManual)}, nilPost: true},
	}
	for _, tc := range cases {
		p := tc.c.Poster()
		if tc.nilPost {
			if p != nil {
				t.Errorf("%s: poster = %+v", tc.name, p)
			}
		} else if p == nil || p.URL != tc.wantURL || p.Source != *tc.c.PosterSource {
			t.Errorf("%s: poster = %+v", tc.name, p)
		}
		if tc.c.PosterRemoved() != tc.removed {
			t.Errorf("%s: removed = %v", tc.name, tc.c.PosterRemoved())
		}
	}
}
