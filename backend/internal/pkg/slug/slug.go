// Package slug turns free text into URL-safe slugs.
package slug

import (
	"regexp"
	"strings"
	"unicode"
)

var unsafeChars = regexp.MustCompile(`[^a-z0-9\-]+`)
var multipleDashes = regexp.MustCompile(`-+`)

var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// Make lowercases s, transliterates common Cyrillic letters and collapses
// everything else that is not a letter or digit into single hyphens.
func Make(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if repl, ok := cyrillic[r]; ok {
			b.WriteString(repl)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	s = b.String()
	s = unsafeChars.ReplaceAllString(s, "-")
	s = multipleDashes.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
