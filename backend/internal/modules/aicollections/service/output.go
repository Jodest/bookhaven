package service

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/bookhaven/backend/internal/pkg/textnorm"
)

type modelBook struct {
	Title         *string `json:"title"`
	Author        *string `json:"author"`
	OriginalTitle *string `json:"original_title"`
	Note          *string `json:"note"`
}

type modelAnswer struct {
	Books *[]modelBook `json:"books"`
}

// ParseOutput validates a model response and returns the cleaned proposals,
// or a non-empty domain error code, in this order: a refusal →
// model_refusal; a truncated answer → model_truncated; an answer that does
// not match the format (unknown or missing field, trailing data) →
// invalid_output. Otherwise every field is stripped of control characters,
// trimmed and whitespace-collapsed; surrounding quotes are stripped from the
// title; entries with an empty title, or a title or author over their caps,
// are dropped; an overlong original title is cleared; notes are cut to
// domain.MaxNoteRunes runes; repeats of a title and author are dropped.
// The rank of a proposal is its position after cleaning; the model never
// numbers its answer. An empty list is valid.
func ParseOutput(resp llm.StructuredResponse) ([]domain.NewProposal, string) {
	switch resp.StopReason {
	case llm.StopReasonRefusal:
		return nil, domain.ErrorModelRefusal
	case llm.StopReasonMaxTokens:
		return nil, domain.ErrorModelTruncated
	}

	var ans modelAnswer
	dec := json.NewDecoder(bytes.NewReader(resp.JSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ans); err != nil {
		return nil, domain.ErrorInvalidOutput
	}
	if _, err := dec.Token(); err != io.EOF {
		// Trailing data after the object.
		return nil, domain.ErrorInvalidOutput
	}
	if ans.Books == nil {
		return nil, domain.ErrorInvalidOutput
	}
	for _, b := range *ans.Books {
		if b.Title == nil || b.Author == nil || b.OriginalTitle == nil || b.Note == nil {
			return nil, domain.ErrorInvalidOutput
		}
	}

	out := make([]domain.NewProposal, 0, len(*ans.Books))
	seen := make(map[string]struct{}, len(*ans.Books))
	for _, b := range *ans.Books {
		title := trimQuotes(cleanText(*b.Title))
		author := cleanText(*b.Author)
		if title == "" || runeLen(title) > domain.MaxTitleRunes || runeLen(author) > domain.MaxAuthorRunes {
			continue
		}
		key := DedupKey(title, author)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		orig := trimQuotes(cleanText(*b.OriginalTitle))
		if runeLen(orig) > domain.MaxTitleRunes {
			orig = ""
		}
		note := cleanText(*b.Note)
		if runes := []rune(note); len(runes) > domain.MaxNoteRunes {
			note = strings.TrimSpace(string(runes[:domain.MaxNoteRunes]))
		}
		out = append(out, domain.NewProposal{Title: title, Author: author, OriginalTitle: orig, Note: note})
	}
	return out, ""
}

// DedupKey is the identity of a proposal for repeat detection.
func DedupKey(title, author string) string {
	return textnorm.Key(title + " " + author)
}

func runeLen(s string) int { return len([]rune(s)) }

// quoteChars are the quotes stripped from both ends of a title.
const quoteChars = "«»\"“”„"

func trimQuotes(s string) string {
	return strings.TrimSpace(strings.Trim(s, quoteChars))
}

// cleanText replaces control characters with spaces, then trims and
// collapses whitespace. Markup is kept as literal text (it is only ever
// rendered as plain text).
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
