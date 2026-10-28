package domain

import "testing"

func strp(s string) *string { return &s }

func copyCols(source, url, status, copySrc string) PosterColumns {
	return PosterColumns{
		PosterSource: strp(source), PosterURL: strp(url),
		PosterCopyPath: strp("ab/abc.jpg"), PosterCopyStatus: strp(status), PosterCopySourceURL: strp(copySrc),
	}
}

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
		{name: "tmdb", c: PosterColumns{PosterSource: strp(PosterSourceTMDB), PosterURL: strp("https://image.tmdb.org/t/p/w500/a.jpg")}, wantURL: "https://image.tmdb.org/t/p/w500/a.jpg"},
		{name: "commons stored copy of current link", c: copyCols(PosterSourceCommons, "https://u/x.jpg", "stored", "https://u/x.jpg"), wantURL: "/covers/ab/abc.jpg"},
		{name: "tmdb stored copy of current link", c: copyCols(PosterSourceTMDB, "https://image.tmdb.org/t/p/w500/a.jpg", "stored", "https://image.tmdb.org/t/p/w500/a.jpg"), wantURL: "/covers/ab/abc.jpg"},
		{name: "stored copy of an old link", c: copyCols(PosterSourceCommons, "https://u/new.jpg", "stored", "https://u/old.jpg"), wantURL: "https://u/new.jpg"},
		{name: "pending copy", c: copyCols(PosterSourceCommons, "https://u/x.jpg", "pending", "https://u/x.jpg"), wantURL: "https://u/x.jpg"},
		{name: "failed copy", c: copyCols(PosterSourceTMDB, "https://u/x.jpg", "failed", "https://u/x.jpg"), wantURL: "https://u/x.jpg"},
		{name: "manual ignores copy columns", c: PosterColumns{PosterSource: strp(PosterSourceManual), PosterPath: strp("ab/abc.jpg"), PosterCopyPath: strp("zz/z.jpg"), PosterCopyStatus: strp("stored")}, wantURL: "/posters/ab/abc.jpg"},
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
