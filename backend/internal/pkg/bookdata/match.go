package bookdata

import (
	"strings"
	"unicode"
)

// NormalizeMatchText lower-cases s, folds «ё» to «е», drops every rune that is
// not a letter, digit or space, collapses whitespace and trims. It is the
// basis of the strict title/author matching used to accept Open Library
// search results.
func NormalizeMatchText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var titleSubtitleSeparators = []string{":", "(", " — ", " - "}

// TitleMatches reports whether candidate equals one of the wanted titles after
// normalization. The part of the candidate before the first subtitle separator
// (":", "(", " — ", " - ") is also accepted.
func TitleMatches(candidate string, wanted []string) bool {
	variants := []string{NormalizeMatchText(candidate)}
	cut := len(candidate)
	for _, sep := range titleSubtitleSeparators {
		if i := strings.Index(candidate, sep); i >= 0 && i < cut {
			cut = i
		}
	}
	if cut < len(candidate) {
		variants = append(variants, NormalizeMatchText(candidate[:cut]))
	}
	for _, w := range wanted {
		nw := NormalizeMatchText(w)
		if nw == "" {
			continue
		}
		for _, v := range variants {
			if v == nw {
				return true
			}
		}
	}
	return false
}

func lastWord(name string) string {
	f := strings.Fields(NormalizeMatchText(name))
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

// AuthorMatches reports whether some candidate author shares its surname (the
// last normalized word) with some wanted author. Empty lists never match.
func AuthorMatches(candidate []string, wanted []string) bool {
	for _, c := range candidate {
		lc := lastWord(c)
		if lc == "" {
			continue
		}
		for _, w := range wanted {
			if lc == lastWord(w) {
				return true
			}
		}
	}
	return false
}
