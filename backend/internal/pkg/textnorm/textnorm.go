// Package textnorm holds the text key shared by catalog matching code.
package textnorm

import (
	"strings"
	"unicode"
)

// Key lowercases s, folds ё to е, turns every rune that is not a letter or
// digit into a space, collapses spaces and trims.
func Key(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		switch {
		case r == 'ё':
			b.WriteRune('е')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
