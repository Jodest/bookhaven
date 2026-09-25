package service

import "unicode"

// cyrillicRatioThreshold is the minimum share of Cyrillic letters (among all
// letters) a string must have for IsRussianText to consider it Russian.
const cyrillicRatioThreshold = 0.5

// IsRussianText reports whether s appears to be written in Russian, via a
// pragmatic Cyrillic-character-ratio heuristic: among s's letters
// (unicode.IsLetter), at least cyrillicRatioThreshold must be Cyrillic
// (unicode.Is(unicode.Cyrillic, r)). Non-letter runes (digits, punctuation,
// whitespace) are ignored and don't count toward the ratio. A string with
// no letters at all (empty, or all digits/punctuation) passes - there is no
// text to judge, so it is not rejected on this basis.
//
// This is a pragmatic MVP choice, not a linguistically rigorous language
// detector: it cannot distinguish Russian from other Cyrillic-script
// languages (Ukrainian, Bulgarian, Serbian, etc.). See
// docs/features/russian-only-content.md Business Rules for the rationale.
//
// Exported so harvest/service can pre-filter enrichment text (see
// handler.go's enrichRecord) before it ever reaches CreateBook.
func IsRussianText(s string) bool {
	var letters, cyrillic int
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.Is(unicode.Cyrillic, r) {
			cyrillic++
		}
	}
	if letters == 0 {
		return true
	}
	return float64(cyrillic)/float64(letters) >= cyrillicRatioThreshold
}
