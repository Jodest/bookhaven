package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
)

// Outcome is a validated model answer ready to store.
type Outcome struct {
	// Status is draft, unknown or failed.
	Status string
	// Text is the normalised description (empty unless draft).
	Text string
	// ErrorCode is set for failed.
	ErrorCode string
}

// modelAnswer is the decoding target; pointers detect missing fields.
type modelAnswer struct {
	Known       *bool   `json:"known"`
	Description *string `json:"description"`
}

func failed(code string) Outcome {
	return Outcome{Status: domain.StatusFailed, ErrorCode: code}
}

// ValidateResponse turns a model response into the outcome to store, in
// this order: a refusal → failed/model_refusal; a truncated answer →
// failed/model_truncated; an answer that does not match the format
// (unknown or missing field, trailing data) → failed/invalid_output;
// known=false → unknown when the text is empty, otherwise invalid_output;
// known=true → the text is normalised (NormalizeText) and must be
// MinTextRunes..MaxTextRunes runes and mainly Cyrillic, otherwise
// invalid_output; otherwise a draft.
func ValidateResponse(resp llm.StructuredResponse) Outcome {
	switch resp.StopReason {
	case llm.StopReasonRefusal:
		return failed(domain.ErrorModelRefusal)
	case llm.StopReasonMaxTokens:
		return failed(domain.ErrorModelTruncated)
	}

	var ans modelAnswer
	dec := json.NewDecoder(bytes.NewReader(resp.JSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ans); err != nil {
		return failed(domain.ErrorInvalidOutput)
	}
	if _, err := dec.Token(); err != io.EOF {
		// Trailing data after the object.
		return failed(domain.ErrorInvalidOutput)
	}
	if ans.Known == nil || ans.Description == nil {
		return failed(domain.ErrorInvalidOutput)
	}

	text := NormalizeText(*ans.Description)
	if !*ans.Known {
		if text != "" {
			return failed(domain.ErrorInvalidOutput)
		}
		return Outcome{Status: domain.StatusUnknown}
	}
	n := utf8.RuneCountInString(text)
	if n < domain.MinTextRunes || n > domain.MaxTextRunes || !adminservice.IsRussianText(text) {
		return failed(domain.ErrorInvalidOutput)
	}
	return Outcome{Status: domain.StatusDraft, Text: text}
}

// ErrInvalidText: an edited text is empty, too long or not mainly
// Cyrillic after normalisation (422).
var ErrInvalidText = errors.New("text must be 1-2000 characters of mainly Cyrillic text")

// ValidateEditedText normalises an admin's text and checks it: 1 to
// MaxEditedRunes runes and mainly Cyrillic.
func ValidateEditedText(s string) (string, error) {
	text := NormalizeText(s)
	n := utf8.RuneCountInString(text)
	if n < 1 || n > domain.MaxEditedRunes || !adminservice.IsRussianText(text) {
		return "", ErrInvalidText
	}
	return text, nil
}

// NormalizeText replaces control characters with spaces, then trims and
// collapses whitespace. Markup is kept as literal text (it is only ever
// rendered as plain text).
func NormalizeText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
