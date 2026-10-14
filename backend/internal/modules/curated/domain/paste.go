package domain

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/textnorm"
	"github.com/google/uuid"
)

// Paste limits.
const (
	// MaxPasteLines is the most non-blank lines in one paste.
	MaxPasteLines = 500
	// MaxPasteChars is the longest accepted pasted text, in characters.
	MaxPasteChars = 100000
	// MaxPasteLineRunes is the length a single line is truncated to.
	MaxPasteLineRunes = 500
	// MaxCandidates caps the candidates returned for one line.
	MaxCandidates = 10
)

// PasteStatus is the outcome of matching one pasted line.
type PasteStatus string

const (
	PasteFound        PasteStatus = "found"
	PasteAmbiguous    PasteStatus = "ambiguous"
	PasteNotFound     PasteStatus = "not_found"
	PasteRejected     PasteStatus = "rejected"
	PasteInCollection PasteStatus = "in_collection"
	PasteDuplicate    PasteStatus = "duplicate"
	PasteInvalid      PasteStatus = "invalid"
)

// ParsedLine is a pasted line split into title and author.
type ParsedLine struct {
	Title  string
	Author string
	// Whole is the line without numbering, used as a title-only retry when
	// the split title matches nothing (a dash can be part of a title).
	Whole string
}

var numberingRe = regexp.MustCompile(`^\s*(№\s*)?\d{1,4}\s*[.)]\s*`)
var numericRe = regexp.MustCompile(`^\s*(№\s*)?\d{1,4}\s*$`)

var lineSeparators = []string{" — ", " – ", " - ", " / "}

const quoteSet = " \t«»\"„“”'"

// cleanPart removes surrounding quotes, spaces and trailing punctuation.
func cleanPart(s string) string {
	for {
		t := strings.Trim(s, quoteSet)
		t = strings.TrimRight(t, ".,;:")
		if t == s {
			return t
		}
		s = t
	}
}

// ParseLine reads one pasted line as title first, author second. ok is false
// when no title remains.
func ParseLine(raw string) (title, author string, ok bool) {
	p := Parse(raw)
	return p.Title, p.Author, p.Title != ""
}

// Parse is ParseLine with the title-only retry text.
func Parse(raw string) ParsedLine {
	line := strings.TrimSpace(truncateRunes(raw, MaxPasteLineRunes))
	if strings.Contains(line, "\t") {
		cells := strings.Split(line, "\t")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) >= 3 && numericRe.MatchString(cells[0]) {
			return ParsedLine{
				Title:  cleanPart(cells[1]),
				Author: cleanPart(strings.Join(nonEmpty(cells[2:]), " ")),
				Whole:  cleanPart(strings.Join(nonEmpty(cells[1:]), " ")),
			}
		}
		first := numberingRe.ReplaceAllString(cells[0], "")
		rest := strings.Join(nonEmpty(cells[1:]), " ")
		return ParsedLine{
			Title: cleanPart(first), Author: cleanPart(rest),
			Whole: cleanPart(strings.TrimSpace(first + " " + rest)),
		}
	}
	line = strings.TrimSpace(numberingRe.ReplaceAllString(line, ""))
	cut, sepLen := -1, 0
	for _, sep := range lineSeparators {
		if i := strings.LastIndex(line, sep); i > cut {
			cut, sepLen = i, len(sep)
		}
	}
	if cut < 0 {
		t := cleanPart(line)
		return ParsedLine{Title: t, Whole: t}
	}
	return ParsedLine{
		Title: cleanPart(line[:cut]), Author: cleanPart(line[cut+sepLen:]), Whole: cleanPart(line),
	}
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// TruncateLine cuts a pasted line to MaxPasteLineRunes runes.
func TruncateLine(s string) string { return truncateRunes(s, MaxPasteLineRunes) }

var authorSplitRe = regexp.MustCompile(`\s*[,;&]\s*|\s+[иИ]\s+`)

// MatchAuthor reports whether any pasted name (names separated by , ; & or
// "и") matches any of the authors: every word of the name equals a word of
// the author, or is one letter equal to such a word's first letter.
func MatchAuthor(pasted string, authorNames []string) bool {
	for _, name := range authorSplitRe.Split(pasted, -1) {
		words := strings.Fields(textnorm.Key(name))
		if len(words) == 0 {
			continue
		}
		for _, a := range authorNames {
			aw := strings.Fields(textnorm.Key(a))
			if len(aw) > 0 && allWordsMatch(words, aw) {
				return true
			}
		}
	}
	return false
}

func allWordsMatch(words, authorWords []string) bool {
	for _, w := range words {
		found := false
		for _, a := range authorWords {
			if a == w || (utf8.RuneCountInString(w) == 1 && strings.HasPrefix(a, w)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Classification is the outcome of matching a line against its title
// matches.
type Classification struct {
	Status     PasteStatus
	Book       *catalogDomain.Book
	Candidates []catalogDomain.Book
}

func statusRank(s string) int {
	switch s {
	case catalogDomain.BookStatusPublished:
		return 0
	case catalogDomain.BookStatusRejected:
		return 2
	}
	return 1
}

// SortCandidates orders books published, pending, rejected, then by title
// and year (unknown year last), then id, and caps the list at MaxCandidates.
func SortCandidates(books []catalogDomain.Book) []catalogDomain.Book {
	out := append([]catalogDomain.Book(nil), books...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := statusRank(a.Status), statusRank(b.Status); ra != rb {
			return ra < rb
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		if (a.PublishedYear == nil) != (b.PublishedYear == nil) {
			return a.PublishedYear != nil
		}
		if a.PublishedYear != nil && *a.PublishedYear != *b.PublishedYear {
			return *a.PublishedYear < *b.PublishedYear
		}
		return a.ID.String() < b.ID.String()
	})
	if len(out) > MaxCandidates {
		out = out[:MaxCandidates]
	}
	return out
}

func authorNamesOf(b catalogDomain.Book) []string {
	names := make([]string, 0, len(b.Authors))
	for _, a := range b.Authors {
		names = append(names, a.Name)
	}
	return names
}

func nonRejected(books []catalogDomain.Book) []catalogDomain.Book {
	var out []catalogDomain.Book
	for _, b := range books {
		if b.Status != catalogDomain.BookStatusRejected {
			out = append(out, b)
		}
	}
	return out
}

// Classify decides the status of a line given the books whose title equals
// the pasted title (titleMatches), the pasted author ("" for none) and the
// ids already in the collection. No title match yields PasteNotFound.
func Classify(titleMatches []catalogDomain.Book, author string, inCollection map[uuid.UUID]bool) Classification {
	if len(titleMatches) == 0 {
		return Classification{Status: PasteNotFound}
	}
	narrowed := titleMatches
	if strings.TrimSpace(author) != "" {
		narrowed = nil
		for _, b := range titleMatches {
			if MatchAuthor(author, authorNamesOf(b)) {
				narrowed = append(narrowed, b)
			}
		}
	}
	r := nonRejected(narrowed)
	if len(r) == 0 {
		if len(nonRejected(titleMatches)) > 0 {
			return Classification{Status: PasteAmbiguous, Candidates: SortCandidates(titleMatches)}
		}
		return Classification{Status: PasteRejected, Candidates: SortCandidates(titleMatches)}
	}
	all := true
	for _, b := range r {
		if !inCollection[b.ID] {
			all = false
			break
		}
	}
	if all {
		c := Classification{Status: PasteInCollection}
		if len(r) == 1 {
			b := r[0]
			c.Book = &b
		}
		return c
	}
	if len(r) == 1 {
		b := r[0]
		return Classification{Status: PasteFound, Book: &b}
	}
	return Classification{Status: PasteAmbiguous, Candidates: SortCandidates(narrowed)}
}

// PasteEntry is the part of a report entry that duplicate marking needs.
type PasteEntry struct {
	Line   int
	Key    string // normalized text key; "" never counts as a duplicate
	Status PasteStatus
	BookID uuid.UUID // non-nil for a resolved book
}

// MarkDuplicates returns, per entry index, the earlier line that the entry
// repeats (0 when it is not a duplicate). An entry repeats an earlier one
// when it resolved to the same book or has the same text key.
func MarkDuplicates(entries []PasteEntry) []int {
	out := make([]int, len(entries))
	byBook := map[uuid.UUID]int{}
	byKey := map[string]int{}
	for i, e := range entries {
		if e.Status == PasteInvalid {
			continue
		}
		if e.BookID != uuid.Nil {
			if line, ok := byBook[e.BookID]; ok {
				out[i] = line
				continue
			}
		}
		if e.Key != "" {
			if line, ok := byKey[e.Key]; ok {
				out[i] = line
				continue
			}
		}
		if e.BookID != uuid.Nil {
			byBook[e.BookID] = e.Line
		}
		if e.Key != "" {
			byKey[e.Key] = e.Line
		}
	}
	return out
}
