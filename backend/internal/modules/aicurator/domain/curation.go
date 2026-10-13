// Package domain holds the types and limits of the AI curator of author
// import lists (openspec capability catalog-import/ai-author-curation). A
// request is an admin's free-text description of missing authors; its
// candidates are the author names the model proposed, each resolved
// against Wikidata. None of this is catalog data: the admin takes
// candidates only through the existing import flows.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// PromptVersion is the current curation prompt version. Bump it whenever
// the system prompt, the output schema or the input shape changes.
const PromptVersion = 1

// Limits.
const (
	// MaxPromptRunes caps the admin's prompt (after trim).
	MaxPromptRunes = 500
	// MaxNames / DefaultNames bound max_names (1..MaxNames).
	MaxNames     = 30
	DefaultNames = 20
	// DailyLimit is the number of requests per admin per rolling 24 h,
	// counting every stored request (failed ones too).
	DailyLimit = 20
	// ActiveStaleAfter: a queued/running request without progress for
	// longer than this is no longer active and reads as failed/timeout.
	ActiveStaleAfter = time.Hour
	// RetentionDays: requests older than this are purged.
	RetentionDays = 30
	// ListLimit is the size of the recent requests list.
	ListLimit = 20
	// MaxNameRunes / MaxNoteRunes bound the stored candidate fields.
	MaxNameRunes = 150
	MaxNoteRunes = 200
)

// Request statuses.
const (
	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// Candidate match statuses.
const (
	MatchPending   = "pending"
	MatchMatched   = "matched"
	MatchAmbiguous = "ambiguous"
	MatchNotFound  = "not_found"
	MatchError     = "error"
)

// Request error codes.
const (
	ErrorModelRefusal   = "model_refusal"
	ErrorModelTruncated = "model_truncated"
	ErrorInvalidOutput  = "invalid_output"
	ErrorModelError     = "model_error"
	ErrorEnqueueFailed  = "enqueue_failed"
	// ErrorTimeout is a read-side projection only; it is never stored.
	ErrorTimeout = "timeout"
)

// IsActiveStatus reports whether status is queued or running.
func IsActiveStatus(status string) bool {
	return status == StatusQueued || status == StatusRunning
}

// Option is one Wikidata person a candidate name resolved to.
type Option struct {
	QID         string `json:"qid"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// NewCandidate is one cleaned name from the model's answer.
type NewCandidate struct {
	Name string
	Note string
}

// Candidate is one stored proposed name.
type Candidate struct {
	ID          uuid.UUID
	RequestID   uuid.UUID
	Position    int
	Name        string
	Note        string
	MatchStatus string
	// Options is never nil.
	Options   []Option
	UpdatedAt time.Time
}

// Creator is the admin who created a request.
type Creator struct {
	ID       uuid.UUID
	Username string
}

// Request is one stored curation request.
type Request struct {
	ID            uuid.UUID
	Prompt        string
	MaxNames      int
	Status        string
	ErrorCode     *string
	Model         string
	PromptVersion int
	InputTokens   int
	OutputTokens  int
	CreatedBy     *uuid.UUID
	// Creator is nil when the creator was deleted.
	Creator    *Creator
	CreatedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time

	CandidatesTotal    int
	CandidatesResolved int
	// Candidates are loaded only by the detail read, in position order.
	Candidates []Candidate
}

// IsStale reports whether an active request made no progress since
// staleBefore (updated_at <= staleBefore).
func (r *Request) IsStale(staleBefore time.Time) bool {
	return IsActiveStatus(r.Status) && !r.UpdatedAt.After(staleBefore)
}
