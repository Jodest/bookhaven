// Package domain holds the types and limits of AI-generated book
// description drafts (openspec capability catalog-admin/ai-descriptions).
// An AI description is kept apart from the book's own description: it is
// never copied into books.description, and it is public only when an admin
// approved it and the book still has no description of its own.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// PromptVersion is the current generation prompt version. Bump it whenever
// the system prompt, the output schema or the model input shape changes.
const PromptVersion = 1

// Limits.
const (
	// MaxBulk caps one all_missing run.
	MaxBulk = 200
	// MaxIDs caps the distinct book ids of one generate request.
	MaxIDs = 100
	// MinTextRunes / MaxTextRunes bound a generated text (after
	// normalisation).
	MinTextRunes = 40
	MaxTextRunes = 1000
	// MaxEditedRunes bounds an admin-edited text (1..MaxEditedRunes after
	// normalisation).
	MaxEditedRunes = 2000
	// DefaultListLimit / MaxListLimit bound the review list page size.
	DefaultListLimit = 20
	MaxListLimit     = 50
)

// Statuses.
const (
	StatusDraft     = "draft"
	StatusApproved  = "approved"
	StatusDiscarded = "discarded"
	StatusUnknown   = "unknown"
	StatusFailed    = "failed"
)

// Statuses lists every status (the list filter accepts exactly these).
var Statuses = []string{StatusDraft, StatusApproved, StatusDiscarded, StatusUnknown, StatusFailed}

// IsStatus reports whether s is a known status.
func IsStatus(s string) bool {
	for _, v := range Statuses {
		if v == s {
			return true
		}
	}
	return false
}

// IsReviewable reports whether a record in status s can be approved or
// discarded.
func IsReviewable(s string) bool {
	return s == StatusDraft || s == StatusApproved
}

// Failure codes of a failed record.
const (
	ErrorModelRefusal   = "model_refusal"
	ErrorModelTruncated = "model_truncated"
	ErrorInvalidOutput  = "invalid_output"
)

// Description is one stored AI description (at most one per book).
type Description struct {
	BookID        uuid.UUID
	Status        string
	Text          string
	ErrorCode     *string
	Edited        bool
	Model         string
	PromptVersion int
	InputHash     string
	InputTokens   int
	OutputTokens  int
	GeneratedAt   time.Time
	ReviewedBy    *uuid.UUID
	ReviewedAt    *time.Time
}

// BookCard is the book shown next to an AI description in the admin app.
type BookCard struct {
	ID            uuid.UUID
	Title         string
	Authors       []string
	PublishedYear *int
	Status        string
}

// Item is an AI description as the admin app sees it. Stale: the prompt
// version or the book's model input changed since generation. Superseded:
// the book now has its own description (the AI one is not public).
type Item struct {
	Description
	Book       BookCard
	Stale      bool
	Superseded bool
}
