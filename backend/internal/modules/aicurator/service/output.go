package service

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
)

type modelAuthor struct {
	Name *string `json:"name"`
	Note *string `json:"note"`
}

type modelAnswer struct {
	Authors *[]modelAuthor `json:"authors"`
}

// ParseOutput validates a model response and returns the cleaned names, or
// a non-empty domain error code, in this order: a refusal →
// model_refusal; a truncated answer → model_truncated; an answer that does
// not match the format (unknown or missing field, trailing data) →
// invalid_output. Otherwise names and notes are stripped of control
// characters, trimmed and whitespace-collapsed; empty names and names over
// domain.MaxNameRunes runes are dropped; duplicates (case-insensitive) are
// dropped; the first maxNames are kept and notes are cut to
// domain.MaxNoteRunes runes. An empty list is valid (no candidates).
func ParseOutput(resp llm.StructuredResponse, maxNames int) ([]domain.NewCandidate, string) {
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
	if ans.Authors == nil {
		return nil, domain.ErrorInvalidOutput
	}
	for _, a := range *ans.Authors {
		if a.Name == nil || a.Note == nil {
			return nil, domain.ErrorInvalidOutput
		}
	}

	out := make([]domain.NewCandidate, 0, len(*ans.Authors))
	seen := make(map[string]struct{}, len(*ans.Authors))
	for _, a := range *ans.Authors {
		if len(out) >= maxNames {
			break
		}
		name := cleanText(*a.Name)
		if name == "" || len([]rune(name)) > domain.MaxNameRunes {
			continue
		}
		key := strings.ToLower(name)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		note := cleanText(*a.Note)
		if runes := []rune(note); len(runes) > domain.MaxNoteRunes {
			note = strings.TrimSpace(string(runes[:domain.MaxNoteRunes]))
		}
		out = append(out, domain.NewCandidate{Name: name, Note: note})
	}
	return out, ""
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
