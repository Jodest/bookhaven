// Package domain holds the types and limits of the AI genre suggestions
// (openspec capability catalog-moderation/ai-genre-suggestions). A suggestion
// is a proposal to merge genres or to reject a junk genre; it is stored apart
// from the genres and applied only by an admin through the existing genre
// merge and bulk status.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PromptVersion is the current prompt version. Bump it whenever the system
// prompt, the output schema or the input shape changes.
const PromptVersion = 1

// Suggestion kinds.
const (
	KindDuplicate = "duplicate"
	KindNarrow    = "narrow"
	KindJunk      = "junk"
)

// Kinds lists the suggestion kinds.
var Kinds = []string{KindDuplicate, KindNarrow, KindJunk}

// IsKind reports whether k is a suggestion kind.
func IsKind(k string) bool {
	return k == KindDuplicate || k == KindNarrow || k == KindJunk
}

// IsMergeKind reports whether the kind merges genres (duplicate or narrow).
func IsMergeKind(k string) bool { return k == KindDuplicate || k == KindNarrow }

// Suggestion statuses.
const (
	StatusOpen      = "open"
	StatusApplied   = "applied"
	StatusDismissed = "dismissed"
	StatusStale     = "stale"
)

// IsStatus reports whether s is a suggestion status.
func IsStatus(s string) bool {
	switch s {
	case StatusOpen, StatusApplied, StatusDismissed, StatusStale:
		return true
	}
	return false
}

// Reason codes.
const (
	ReasonSpelling  = "spelling"
	ReasonLanguage  = "language"
	ReasonSynonym   = "synonym"
	ReasonTooNarrow = "too_narrow"
	ReasonNotAGenre = "not_a_genre"
	ReasonDate      = "date"
	ReasonMarketing = "marketing"
	ReasonFormat    = "format"
)

// ReasonCodesFor returns the codes allowed for a kind.
func ReasonCodesFor(kind string) []string {
	switch kind {
	case KindDuplicate:
		return []string{ReasonSpelling, ReasonLanguage, ReasonSynonym}
	case KindNarrow:
		return []string{ReasonTooNarrow}
	case KindJunk:
		return []string{ReasonNotAGenre, ReasonDate, ReasonMarketing, ReasonFormat}
	}
	return nil
}

// AllReasonCodes lists every reason code (the output schema enum).
func AllReasonCodes() []string {
	return []string{
		ReasonSpelling, ReasonLanguage, ReasonSynonym, ReasonTooNarrow,
		ReasonNotAGenre, ReasonDate, ReasonMarketing, ReasonFormat,
	}
}

// IsReasonCodeOf reports whether code is allowed for the kind.
func IsReasonCodeOf(kind, code string) bool {
	for _, c := range ReasonCodesFor(kind) {
		if c == code {
			return true
		}
	}
	return false
}

// Roles of a genre inside a suggestion.
const (
	RoleTarget = "target"
	RoleSource = "source"
)

// Limits.
const (
	// MaxGenres is the most genres (target included) in one suggestion.
	MaxGenres = 20
	// MaxReasonRunes caps the stored reason.
	MaxReasonRunes = 300
	// MinConfidence: items below it are dropped.
	MinConfidence = 0.5
	// MaxItemsPerAnswer caps the suggestions stored per model answer.
	MaxItemsPerAnswer = 50
	// NarrowMaxSourceBooks: a narrow source may have at most this many books.
	NarrowMaxSourceBooks = 10
	// AnchorCount is the number of most popular published genres sent with
	// every request.
	AnchorCount = 100
	// MaxBatches is the most model requests in one run.
	MaxBatches = 10
	// MaxClusterSize bounds a hinted group kept in one batch.
	MaxClusterSize = 50
	// SampleBooksPerGenre and SampleTitleRunes bound the sample books sent.
	SampleBooksPerGenre = 2
	SampleTitleRunes    = 80
	// ActiveStaleAfter: a queued/running run older than this is interrupted.
	ActiveStaleAfter = 2 * time.Hour
	// RunRetention: runs older than this are purged at the start of a run.
	RunRetention = 180 * 24 * time.Hour
	// ListDefaultLimit / ListMaxLimit bound the suggestion list page.
	ListDefaultLimit = 20
	ListMaxLimit     = 50
	// BulkMax bounds one bulk apply/dismiss request.
	BulkMax = 50
)

// Run statuses.
const (
	RunQueued  = "queued"
	RunRunning = "running"
	RunDone    = "done"
	RunFailed  = "failed"
)

// Run triggers.
const (
	TriggerManual    = "manual"
	TriggerScheduled = "scheduled"
)

// Run error codes.
const (
	ErrorModelRefusal   = "model_refusal"
	ErrorModelTruncated = "model_truncated"
	ErrorInvalidOutput  = "invalid_output"
	ErrorModelError     = "model_error"
	ErrorTransient      = "transient"
	ErrorEnqueueFailed  = "enqueue_failed"
	ErrorInterrupted    = "interrupted"
	ErrorInternal       = "internal"
)

// IsActiveRunStatus reports whether status is queued or running.
func IsActiveRunStatus(status string) bool {
	return status == RunQueued || status == RunRunning
}

// Run is one analysis run.
type Run struct {
	ID                 uuid.UUID
	Trigger            string
	Status             string
	ErrorCode          *string
	Model              string
	PromptVersion      int
	Fingerprint        string
	BatchesTotal       int
	BatchesDone        int
	SuggestionsCreated int
	InputTokens        int
	OutputTokens       int
	CreatedBy          *uuid.UUID
	CreatedAt          time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
}

// SuggestionGenre is one genre of a suggestion with the name it had when the
// suggestion was created.
type SuggestionGenre struct {
	GenreID      uuid.UUID
	Role         string
	Position     int
	NameSnapshot string
}

// Suggestion is one stored suggestion. Genres are ordered target first, then
// the sources by position.
type Suggestion struct {
	ID              uuid.UUID
	RunID           *uuid.UUID
	Kind            string
	Status          string
	SetKey          string
	ReasonCodes     []string
	Reason          string
	Confidence      float64
	Model           string
	PromptVersion   int
	AppliedTargetID *uuid.UUID
	DecidedBy       *uuid.UUID
	DecidedAt       *time.Time
	CreatedAt       time.Time
	Genres          []SuggestionGenre
}

// Target returns the target genre of a merge suggestion, or nil.
func (s *Suggestion) Target() *SuggestionGenre {
	for i := range s.Genres {
		if s.Genres[i].Role == RoleTarget {
			return &s.Genres[i]
		}
	}
	return nil
}

// GenreIDs returns the ids of every genre of the suggestion, target first.
func (s *Suggestion) GenreIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(s.Genres))
	for _, g := range s.Genres {
		ids = append(ids, g.GenreID)
	}
	return ids
}

// NewSuggestion is a validated suggestion ready to store.
type NewSuggestion struct {
	Kind          string
	ReasonCodes   []string
	Reason        string
	Confidence    float64
	Model         string
	PromptVersion int
	// Genres holds the target first (merge kinds), then the sources.
	Genres []SuggestionGenre
}

// Action is the action a kind stands for: merge or reject.
func Action(kind string) string {
	if kind == KindJunk {
		return "reject"
	}
	return "merge"
}

// SetKey identifies the set of a suggestion: its action together with all its
// genres, target included, independent of order and of which one is the
// target.
func SetKey(action string, genreIDs []uuid.UUID) string {
	ids := make([]string, 0, len(genreIDs))
	for _, id := range genreIDs {
		ids = append(ids, id.String())
	}
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(action + ":" + strings.Join(ids, ",")))
	return hex.EncodeToString(sum[:])
}

// SetKeyOf returns the set key of a new suggestion.
func (n *NewSuggestion) SetKeyOf() string {
	ids := make([]uuid.UUID, 0, len(n.Genres))
	for _, g := range n.Genres {
		ids = append(ids, g.GenreID)
	}
	return SetKey(Action(n.Kind), ids)
}
