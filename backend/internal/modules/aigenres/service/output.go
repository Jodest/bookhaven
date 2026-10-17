package service

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strings"

	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
)

// refGenre is what Validate knows about a batch-local reference.
type refGenre struct {
	ID    uuid.UUID
	Name  string
	Books int64
}

// Outcome is the result of validating one model answer.
type Outcome struct {
	// Code is "" for a usable answer, otherwise model_refusal,
	// model_truncated or invalid_output; nothing is stored then.
	Code string
	// Items are the validated suggestions, in the model's order.
	Items []domain.NewSuggestion
	// Dropped is the number of items dropped by an item-level rule. Only the
	// count is logged, never names.
	Dropped int
}

type answerItem struct {
	Kind        *string   `json:"kind"`
	Target      *string   `json:"target"`
	Genres      *[]string `json:"genres"`
	ReasonCodes *[]string `json:"reason_codes"`
	Reason      *string   `json:"reason"`
	Confidence  *float64  `json:"confidence"`
}

type modelAnswer struct {
	Suggestions *[]answerItem `json:"suggestions"`
}

// Validate turns a model response into the suggestions to store, in this
// order: a refusal and a truncated answer store nothing (model_refusal,
// model_truncated); an answer that is not the expected JSON or lacks the
// suggestion list stores nothing (invalid_output); otherwise every item is
// checked on its own and dropped when it breaks a rule, at most
// domain.MaxItemsPerAnswer items are kept and a long reason is truncated.
// refs maps the batch-local references to genres; model is stored on every
// suggestion.
func Validate(resp llm.StructuredResponse, refs map[string]refGenre, model string) Outcome {
	switch resp.StopReason {
	case llm.StopReasonRefusal:
		return Outcome{Code: domain.ErrorModelRefusal}
	case llm.StopReasonMaxTokens:
		return Outcome{Code: domain.ErrorModelTruncated}
	}

	var ans modelAnswer
	dec := json.NewDecoder(bytes.NewReader(resp.JSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ans); err != nil {
		return Outcome{Code: domain.ErrorInvalidOutput}
	}
	if _, err := dec.Token(); err != io.EOF {
		return Outcome{Code: domain.ErrorInvalidOutput}
	}
	if ans.Suggestions == nil {
		return Outcome{Code: domain.ErrorInvalidOutput}
	}

	out := Outcome{}
	used := map[string]struct{}{}
	for _, item := range *ans.Suggestions {
		if len(out.Items) >= domain.MaxItemsPerAnswer {
			break
		}
		s, keys, ok := validateItem(item, refs, used, model)
		if !ok {
			out.Dropped++
			continue
		}
		for _, k := range keys {
			used[k] = struct{}{}
		}
		out.Items = append(out.Items, s)
	}
	return out
}

// validateItem checks one item against every per-item rule; ok is false when
// it must be dropped. keys are the refs the item uses.
func validateItem(item answerItem, refs map[string]refGenre, used map[string]struct{}, model string) (domain.NewSuggestion, []string, bool) {
	var zero domain.NewSuggestion
	if item.Kind == nil || item.Genres == nil || item.ReasonCodes == nil || item.Reason == nil || item.Confidence == nil {
		return zero, nil, false
	}
	kind := *item.Kind
	if !domain.IsKind(kind) {
		return zero, nil, false
	}

	conf := *item.Confidence
	if math.IsNaN(conf) || conf < 0 || conf > 1 || conf < domain.MinConfidence {
		return zero, nil, false
	}

	codes := make([]string, 0, len(*item.ReasonCodes))
	seenCode := map[string]struct{}{}
	for _, c := range *item.ReasonCodes {
		if !domain.IsReasonCodeOf(kind, c) {
			return zero, nil, false
		}
		if _, dup := seenCode[c]; dup {
			continue
		}
		seenCode[c] = struct{}{}
		codes = append(codes, c)
	}
	if len(codes) == 0 {
		return zero, nil, false
	}

	// Sources: known refs, none twice.
	seenRef := map[string]struct{}{}
	keys := make([]string, 0, len(*item.Genres)+1)
	sources := make([]refGenre, 0, len(*item.Genres))
	for _, ref := range *item.Genres {
		g, known := refs[ref]
		if !known {
			return zero, nil, false
		}
		if _, dup := seenRef[ref]; dup {
			return zero, nil, false
		}
		seenRef[ref] = struct{}{}
		keys = append(keys, ref)
		sources = append(sources, g)
	}

	var target *refGenre
	if domain.IsMergeKind(kind) {
		if item.Target == nil || len(sources) == 0 || len(sources) > domain.MaxGenres-1 {
			return zero, nil, false
		}
		g, known := refs[*item.Target]
		if !known {
			return zero, nil, false
		}
		if _, inSources := seenRef[*item.Target]; inSources {
			return zero, nil, false
		}
		keys = append(keys, *item.Target)
		target = &g
		if kind == domain.KindNarrow {
			for _, s := range sources {
				if s.Books > domain.NarrowMaxSourceBooks || s.Books >= target.Books {
					return zero, nil, false
				}
			}
		}
	} else if item.Target != nil || len(sources) != 1 {
		return zero, nil, false
	}

	for _, k := range keys {
		if _, taken := used[k]; taken {
			return zero, nil, false
		}
	}

	reason := strings.TrimSpace(*item.Reason)
	if runes := []rune(reason); len(runes) > domain.MaxReasonRunes {
		reason = strings.TrimSpace(string(runes[:domain.MaxReasonRunes]))
	}

	genres := make([]domain.SuggestionGenre, 0, len(sources)+1)
	if target != nil {
		genres = append(genres, domain.SuggestionGenre{GenreID: target.ID, Role: domain.RoleTarget, NameSnapshot: target.Name})
	}
	for _, s := range sources {
		genres = append(genres, domain.SuggestionGenre{GenreID: s.ID, Role: domain.RoleSource, NameSnapshot: s.Name})
	}
	return domain.NewSuggestion{
		Kind: kind, ReasonCodes: codes, Reason: reason, Confidence: math.Round(conf*100) / 100,
		Model: model, PromptVersion: PromptVersion, Genres: genres,
	}, keys, true
}
