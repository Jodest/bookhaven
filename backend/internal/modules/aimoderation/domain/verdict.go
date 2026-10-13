// Package domain holds the AI pre-moderation vocabulary shared by the
// admin module (verdict storage, queue) and the aimoderation service
// (prompt, validation): verdict values, reason codes and the prompt
// version (openspec capability catalog-moderation/ai-verdicts).
package domain

// PromptVersion identifies the system prompt, output schema and model
// input shape. Bump it whenever any of them changes: every stored verdict
// with another version becomes stale and is recomputed by the sweep.
const PromptVersion = 1

// Verdict values.
const (
	VerdictPublish    = "publish"
	VerdictReject     = "reject"
	VerdictNeedsHuman = "needs_human"
)

// Model reason codes (the model may return these).
const (
	ReasonDuplicate        = "duplicate"
	ReasonNotRussian       = "not_russian"
	ReasonNotABook         = "not_a_book"
	ReasonCollection       = "collection"
	ReasonPeriodical       = "periodical"
	ReasonExcerpt          = "excerpt"
	ReasonWrongAuthor      = "wrong_author"
	ReasonGenreMismatch    = "genre_mismatch"
	ReasonInsufficientData = "insufficient_data"
)

// System failure codes (set by validation, never accepted from the model).
const (
	ReasonModelRefusal   = "model_refusal"
	ReasonModelTruncated = "model_truncated"
	ReasonInvalidOutput  = "invalid_output"
)

// Verdicts lists the verdict values in schema order.
var Verdicts = []string{VerdictPublish, VerdictReject, VerdictNeedsHuman}

// ModelReasonCodes lists the codes the model may return, in schema order.
var ModelReasonCodes = []string{
	ReasonDuplicate, ReasonNotRussian, ReasonNotABook, ReasonCollection, ReasonPeriodical,
	ReasonExcerpt, ReasonWrongAuthor, ReasonGenreMismatch, ReasonInsufficientData,
}

// IsVerdict reports whether v is a known verdict value.
func IsVerdict(v string) bool {
	for _, x := range Verdicts {
		if x == v {
			return true
		}
	}
	return false
}

// IsModelReasonCode reports whether code is one the model may return.
func IsModelReasonCode(code string) bool {
	for _, x := range ModelReasonCodes {
		if x == code {
			return true
		}
	}
	return false
}

// IsReasonCode reports whether code is a model or a system failure code.
func IsReasonCode(code string) bool {
	switch code {
	case ReasonModelRefusal, ReasonModelTruncated, ReasonInvalidOutput:
		return true
	}
	return IsModelReasonCode(code)
}
