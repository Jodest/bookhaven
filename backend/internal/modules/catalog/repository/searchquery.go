package repository

import (
	"strings"
	"unicode"
)

// buildPrefixTSQuery turns free-form user input into a to_tsquery string where
// every word is a prefix term, AND-ed together ("брат кол" -> "брат:* & кол:*").
// Everything that is not a letter or digit is treated as a separator, so no
// tsquery operator can reach Postgres. Returns "" when nothing remains.
func buildPrefixTSQuery(input string) string {
	words := strings.FieldsFunc(input, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for i, w := range words {
		words[i] = w + ":*"
	}
	return strings.Join(words, " & ")
}
