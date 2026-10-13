package service

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strings"

	"github.com/bookhaven/backend/internal/modules/aimoderation/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
)

// Outcome is a validated verdict ready to store.
type Outcome struct {
	Verdict     string
	ReasonCodes []string
	Reason      string
	Confidence  float64
}

// Fixed outcomes of answers that cannot be used as they are.
var (
	outcomeRefusal = Outcome{
		Verdict: domain.VerdictNeedsHuman, ReasonCodes: []string{domain.ReasonModelRefusal},
		Reason: "Модель отказалась оценивать книгу.",
	}
	outcomeTruncated = Outcome{
		Verdict: domain.VerdictNeedsHuman, ReasonCodes: []string{domain.ReasonModelTruncated},
		Reason: "Ответ модели обрезан.",
	}
	outcomeInvalid = Outcome{
		Verdict: domain.VerdictNeedsHuman, ReasonCodes: []string{domain.ReasonInvalidOutput},
		Reason: "Некорректный ответ модели.",
	}
)

// modelAnswer is the decoding target; pointers detect missing fields.
type modelAnswer struct {
	Verdict     *string   `json:"verdict"`
	ReasonCodes *[]string `json:"reason_codes"`
	Reason      *string   `json:"reason"`
	Confidence  *float64  `json:"confidence"`
}

// ValidateResponse turns a model response into the verdict to store, in
// this order: a refusal and a truncated answer get fixed needs_human
// outcomes; an answer that does not match the format gets needs_human /
// invalid_output; otherwise the answer is kept with reason codes
// deduplicated, the reason trimmed and cut to 300 runes and the confidence
// rounded to 2 decimals. Every fixed outcome has confidence 0.
func ValidateResponse(resp llm.StructuredResponse) Outcome {
	switch resp.StopReason {
	case llm.StopReasonRefusal:
		return cloneOutcome(outcomeRefusal)
	case llm.StopReasonMaxTokens:
		return cloneOutcome(outcomeTruncated)
	}

	var ans modelAnswer
	dec := json.NewDecoder(bytes.NewReader(resp.JSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ans); err != nil {
		return cloneOutcome(outcomeInvalid)
	}
	if _, err := dec.Token(); err != io.EOF {
		// Trailing data after the object.
		return cloneOutcome(outcomeInvalid)
	}
	if ans.Verdict == nil || ans.ReasonCodes == nil || ans.Reason == nil || ans.Confidence == nil {
		return cloneOutcome(outcomeInvalid)
	}
	if !domain.IsVerdict(*ans.Verdict) {
		return cloneOutcome(outcomeInvalid)
	}
	conf := *ans.Confidence
	if math.IsNaN(conf) || conf < 0 || conf > 1 {
		return cloneOutcome(outcomeInvalid)
	}
	codes := make([]string, 0, len(*ans.ReasonCodes))
	seen := map[string]struct{}{}
	for _, c := range *ans.ReasonCodes {
		if !domain.IsModelReasonCode(c) {
			return cloneOutcome(outcomeInvalid)
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		codes = append(codes, c)
	}
	if *ans.Verdict == domain.VerdictPublish && len(codes) > 0 {
		return cloneOutcome(outcomeInvalid)
	}
	if *ans.Verdict != domain.VerdictPublish && len(codes) == 0 {
		return cloneOutcome(outcomeInvalid)
	}

	reason := strings.TrimSpace(*ans.Reason)
	if runes := []rune(reason); len(runes) > maxReasonRunes {
		reason = strings.TrimSpace(string(runes[:maxReasonRunes]))
	}
	return Outcome{
		Verdict:     *ans.Verdict,
		ReasonCodes: codes,
		Reason:      reason,
		Confidence:  math.Round(conf*100) / 100,
	}
}

func cloneOutcome(o Outcome) Outcome {
	o.ReasonCodes = append([]string(nil), o.ReasonCodes...)
	return o
}
