package main

import (
	"testing"
	"unicode/utf8"
)

// TestDemoReadersFillHomeBlocks guards the fixture promises of
// docs/features/home-page-content.md (Dev data): at least 6 books reach the
// "well-rated" thresholds (rating >= 4.0, ratings_count >= 3) from the demo
// ratings alone, and 3-4 reviews of 40+ characters on different books, each
// matching the reviewer's own rating.
func TestDemoReadersFillHomeBlocks(t *testing.T) {
	sums := map[string]int{}
	counts := map[string]int{}
	reviewed := map[string]bool{}
	reviews := 0
	for i, r := range demoReaders {
		for slug, v := range r.ratings {
			sums[slug] += v
			counts[slug]++
		}
		for slug, content := range r.reviews {
			reviews++
			if reviewed[slug] {
				t.Errorf("reader %d: second review of %s (the home page shows one review per book)", i+1, slug)
			}
			reviewed[slug] = true
			if n := utf8.RuneCountInString(content); n < 40 {
				t.Errorf("reader %d: review of %s is %d characters, want >= 40", i+1, slug, n)
			}
			if v, ok := r.ratings[slug]; !ok || v < 4 {
				t.Errorf("reader %d: review of %s needs a 4-5 rating, got %d (present=%v)", i+1, slug, v, ok)
			}
		}
	}

	wellRated := 0
	for slug, n := range counts {
		if n >= 3 && float64(sums[slug])/float64(n) >= 4.0 {
			wellRated++
		}
	}
	if wellRated < 6 {
		t.Errorf("expected >= 6 well-rated demo books, got %d", wellRated)
	}
	if reviews < 3 || reviews > 4 {
		t.Errorf("expected 3-4 demo reviews, got %d", reviews)
	}
}
