// Package domain holds the types and limits of the AI drafts of manual
// curated collections (openspec capability
// collections/curated-collection-ai-drafts). A draft is an admin's brief and
// the ranked books the model proposed for it. None of this is catalog data:
// the admin applies matched catalog books through the existing bulk add, and
// a proposed book missing from the catalog is only reported.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// PromptVersion is the current draft prompt version. Bump it whenever the
// system prompt, the output schema or the input shape changes.
const PromptVersion = 2

// Limits.
const (
	// MaxBriefRunes caps the admin's brief (after trim).
	MaxBriefRunes = 500
	// MaxCount bounds count (1..MaxCount); equals the curated MaxBooks.
	MaxCount = 1000
	// ChunkSize is the most proposals asked from the model in one call.
	ChunkSize = 100
	// DailyLimit is the number of drafts per admin per rolling 24 h,
	// counting every stored draft (failed ones too).
	DailyLimit = 20
	// ActiveStaleAfter: a queued/running draft without progress for longer
	// than this is no longer active and reads as failed/timeout.
	ActiveStaleAfter = time.Hour
	// RetentionDays: drafts older than this are purged.
	RetentionDays = 30
	// ListLimit is the size of the recent drafts list.
	ListLimit = 20
	// Caps of the stored proposal fields.
	MaxTitleRunes  = 300
	MaxAuthorRunes = 200
	MaxNoteRunes   = 150
)

// Draft statuses.
const (
	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// Draft error codes.
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

// DefaultCount is the number of books asked for when count is omitted: the
// collection size plus a fifth (rounded up), capped at MaxCount.
func DefaultCount(size int) int {
	if size < 0 {
		size = 0
	}
	n := size + (size+4)/5
	if n < 1 {
		n = 1
	}
	if n > MaxCount {
		n = MaxCount
	}
	return n
}

// NewProposal is one cleaned proposal from the model's answer.
type NewProposal struct {
	Title         string
	Author        string
	OriginalTitle string
	Note          string
}

// Proposal is one stored proposal.
type Proposal struct {
	DraftID       uuid.UUID
	Rank          int
	Title         string
	Author        string
	OriginalTitle string
	Note          string
}

// Creator is the admin who created a draft.
type Creator struct {
	ID       uuid.UUID
	Username string
}

// Draft is one stored draft.
type Draft struct {
	ID            uuid.UUID
	CollectionID  uuid.UUID
	Brief         string
	Count         int
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

	// ProposalsCount is the number of stored proposals (the progress).
	ProposalsCount int
}

// IsStale reports whether an active draft made no progress since
// staleBefore (updated_at <= staleBefore).
func (d *Draft) IsStale(staleBefore time.Time) bool {
	return IsActiveStatus(d.Status) && !d.UpdatedAt.After(staleBefore)
}
