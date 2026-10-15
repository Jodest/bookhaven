package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/bookhaven/backend/internal/pkg/textnorm"
	"github.com/google/uuid"
)

const (
	// titleCandidateLimit caps the title candidates fetched per key.
	titleCandidateLimit = 50
	// MaxBulkIDs is the most book ids in one bulk add.
	MaxBulkIDs = 500
)

// Bulk-add skip reasons.
const (
	SkipNotFound     = "not_found"
	SkipRejected     = "rejected"
	SkipAlreadyIn    = "already_in_collection"
	SkipDuplicate    = "duplicate"
	pasteStatusCount = 7
)

// PasteBook is a catalog book in a paste report.
type PasteBook struct {
	Book         catalogDomain.Book
	InCollection bool
}

// PasteLine is one report entry.
type PasteLine struct {
	Line        int
	Text        string
	Title       string
	Author      *string
	Status      domain.PasteStatus
	Book        *PasteBook
	Candidates  []PasteBook
	DuplicateOf *int
}

// PastePreview is the whole report.
type PastePreview struct {
	BooksCount int
	FreeSlots  int
	Counts     map[domain.PasteStatus]int
	Lines      []PasteLine
}

// AddedBook is a book appended by BulkAdd.
type AddedBook struct {
	BookID   uuid.UUID
	Position int
}

// SkippedBook is a requested id that BulkAdd did not add.
type SkippedBook struct {
	BookID uuid.UUID
	Reason string
}

// BulkResult is the outcome of BulkAdd.
type BulkResult struct {
	Added      []AddedBook
	Skipped    []SkippedBook
	BooksCount int
}

type numberedLine struct {
	n    int
	text string
}

func splitPasteLines(text string) ([]numberedLine, error) {
	if utf8.RuneCountInString(text) > domain.MaxPasteChars {
		return nil, invalid("text: must be at most %d characters", domain.MaxPasteChars)
	}
	var out []numberedLine
	for i, raw := range strings.Split(text, "\n") {
		t := strings.TrimSpace(domain.TruncateLine(strings.TrimSpace(raw)))
		if t == "" {
			continue
		}
		out = append(out, numberedLine{n: i + 1, text: t})
	}
	if len(out) == 0 {
		return nil, invalid("text: must contain at least one line")
	}
	if len(out) > domain.MaxPasteLines {
		return nil, domain.ErrTooManyLines
	}
	return out, nil
}

func toPasteBooks(books []catalogDomain.Book, in map[uuid.UUID]bool) []PasteBook {
	out := make([]PasteBook, len(books))
	for i, b := range books {
		out[i] = PasteBook{Book: b, InCollection: in[b.ID]}
	}
	return out
}

// matchTry is one extra exact-title lookup tried when the item's own title
// matches nothing. KeepAuthor keeps the item's author for the classification;
// otherwise the retried text is taken as a whole title with no author.
type matchTry struct {
	Title      string
	KeepAuthor bool
}

// matchItem is one thing to match against the catalog: a pasted line or an
// AI proposal. An empty Title is reported as invalid.
type matchItem struct {
	Line    int
	Text    string
	Title   string
	Author  string
	Key     string
	Retries []matchTry
}

// PreviewPaste matches every pasted line against the catalog. It writes
// nothing and never creates a book.
func (s *Service) PreviewPaste(ctx context.Context, id uuid.UUID, text string) (*PastePreview, error) {
	if err := s.requireManual(ctx, id); err != nil {
		return nil, err
	}
	lines, err := splitPasteLines(text)
	if err != nil {
		return nil, err
	}
	items := make([]matchItem, 0, len(lines))
	for _, ln := range lines {
		p := domain.Parse(ln.text)
		it := matchItem{
			Line: ln.n, Text: ln.text, Title: p.Title, Author: p.Author,
			Key: textnorm.Key(p.Title + " " + p.Author),
		}
		if p.Author != "" {
			it.Retries = []matchTry{{Title: p.Whole}}
		}
		items = append(items, it)
	}
	return s.matchItems(ctx, id, items)
}

// matchItems is the matching core shared by bulk paste and AI proposals.
func (s *Service) matchItems(ctx context.Context, id uuid.UUID, items []matchItem) (*PastePreview, error) {
	heldIDs, err := s.repo.CollectionBookIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("collection books: %w", err)
	}
	held := make(map[uuid.UUID]bool, len(heldIDs))
	for _, b := range heldIDs {
		held[b] = true
	}

	cache := map[string][]catalogDomain.Book{}
	exact := func(key string) ([]catalogDomain.Book, error) {
		if m, ok := cache[key]; ok {
			return m, nil
		}
		cands, err := s.repo.TitleCandidates(ctx, key, titleCandidateLimit)
		if err != nil {
			return nil, fmt.Errorf("title candidates: %w", err)
		}
		var m []catalogDomain.Book
		for _, b := range cands {
			if textnorm.Key(b.Title) == key {
				m = append(m, b)
			}
		}
		cache[key] = m
		return m, nil
	}

	report := make([]PasteLine, 0, len(items))
	entries := make([]domain.PasteEntry, 0, len(items))
	for _, it := range items {
		pl := PasteLine{Line: it.Line, Text: it.Text, Candidates: []PasteBook{}}
		titleKey := textnorm.Key(it.Title)
		if titleKey == "" {
			pl.Status = domain.PasteInvalid
			report = append(report, pl)
			entries = append(entries, domain.PasteEntry{Line: it.Line, Status: domain.PasteInvalid})
			continue
		}
		title, author := it.Title, it.Author
		matches, err := exact(titleKey)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			for _, r := range it.Retries {
				if !r.KeepAuthor && author == "" {
					continue
				}
				rk := textnorm.Key(r.Title)
				if rk == "" || rk == titleKey {
					continue
				}
				if matches, err = exact(rk); err != nil {
					return nil, err
				}
				if len(matches) > 0 {
					if !r.KeepAuthor {
						title, author = r.Title, ""
					}
					break
				}
			}
		}
		cl := domain.Classify(matches, author, held)
		pl.Title, pl.Status = title, cl.Status
		if author != "" {
			a := author
			pl.Author = &a
		}
		entry := domain.PasteEntry{Line: it.Line, Key: it.Key, Status: cl.Status}
		if cl.Book != nil {
			pl.Book = &PasteBook{Book: *cl.Book, InCollection: held[cl.Book.ID]}
			entry.BookID = cl.Book.ID
		}
		if cl.Status == domain.PasteAmbiguous || cl.Status == domain.PasteRejected {
			pl.Candidates = toPasteBooks(cl.Candidates, held)
		}
		report = append(report, pl)
		entries = append(entries, entry)
	}

	counts := make(map[domain.PasteStatus]int, pasteStatusCount)
	for _, st := range []domain.PasteStatus{
		domain.PasteFound, domain.PasteAmbiguous, domain.PasteNotFound, domain.PasteRejected,
		domain.PasteInCollection, domain.PasteDuplicate, domain.PasteInvalid,
	} {
		counts[st] = 0
	}
	for i, dupOf := range domain.MarkDuplicates(entries) {
		if dupOf > 0 {
			d := dupOf
			report[i].Status = domain.PasteDuplicate
			report[i].Book = nil
			report[i].Candidates = []PasteBook{}
			report[i].DuplicateOf = &d
		}
		counts[report[i].Status]++
	}
	free := domain.MaxBooks - len(heldIDs)
	if free < 0 {
		free = 0
	}
	return &PastePreview{BooksCount: len(heldIDs), FreeSlots: free, Counts: counts, Lines: report}, nil
}

// ManualInfo is what AI drafts need to know about a manual collection.
type ManualInfo struct {
	Title       string
	Description string
	GenreName   string
	Size        int
}

// ManualInfo loads a manual collection: ErrCollectionNotFound for an unknown
// one and ErrCollectionAutomatic for an automatic one.
func (s *Service) ManualInfo(ctx context.Context, id uuid.UUID) (*ManualInfo, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, wrapGet(err)
	}
	if c.IsAuto() {
		return nil, domain.ErrCollectionAutomatic
	}
	info := &ManualInfo{Title: c.Title, Description: c.Description, Size: c.Size}
	if c.Genre != nil {
		info.GenreName = c.Genre.Name
	}
	return info, nil
}

// Proposal is one AI-proposed book to match against the catalog.
type Proposal struct {
	Rank          int
	Title         string
	Author        string
	OriginalTitle string
}

// MatchProposals matches AI proposals with the bulk-paste rules, plus a retry
// by the original title that keeps the author. It writes nothing and never
// creates a book.
func (s *Service) MatchProposals(ctx context.Context, id uuid.UUID, ps []Proposal) (*PastePreview, error) {
	if err := s.requireManual(ctx, id); err != nil {
		return nil, err
	}
	items := make([]matchItem, 0, len(ps))
	for _, p := range ps {
		text := p.Title
		if p.Author != "" {
			text = p.Title + " — " + p.Author
		}
		it := matchItem{
			Line: p.Rank, Text: text, Title: p.Title, Author: p.Author,
			Key: textnorm.Key(p.Title + " " + p.Author),
		}
		if p.OriginalTitle != "" {
			it.Retries = []matchTry{{Title: p.OriginalTitle, KeepAuthor: true}}
		}
		items = append(items, it)
	}
	return s.matchItems(ctx, id, items)
}

// BulkAdd appends the chosen catalog books in the given order as one block.
// Unknown, rejected, repeated and already-held ids are skipped; if the rest
// exceeds the free places nothing is added (ErrCollectionFull).
func (s *Service) BulkAdd(ctx context.Context, id uuid.UUID, bookIDs []uuid.UUID, position *int) (*BulkResult, error) {
	if len(bookIDs) == 0 || len(bookIDs) > MaxBulkIDs {
		return nil, invalid("book_ids: must contain 1 to %d ids", MaxBulkIDs)
	}
	if position != nil && *position < 1 {
		return nil, domain.ErrInvalidPosition
	}
	if err := s.requireManual(ctx, id); err != nil {
		return nil, err
	}
	unique := make([]uuid.UUID, 0, len(bookIDs))
	seen := make(map[uuid.UUID]bool, len(bookIDs))
	for _, b := range bookIDs {
		if !seen[b] {
			seen[b] = true
			unique = append(unique, b)
		}
	}
	found, err := s.repo.BooksByIDs(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("books by ids: %w", err)
	}
	status := make(map[uuid.UUID]string, len(found))
	for _, b := range found {
		status[b.ID] = b.Status
	}

	reasons := make([]string, len(bookIDs))
	firstIdx := make(map[uuid.UUID]int, len(unique))
	var addable []uuid.UUID
	for i, b := range bookIDs {
		st, exists := status[b]
		if _, repeat := firstIdx[b]; repeat {
			reasons[i] = SkipDuplicate
			continue
		}
		firstIdx[b] = i
		switch {
		case !exists:
			reasons[i] = SkipNotFound
		case st == catalogDomain.BookStatusRejected:
			reasons[i] = SkipRejected
		default:
			addable = append(addable, b)
		}
	}

	res, err := s.repo.AddBooks(ctx, id, addable, position)
	if err != nil {
		if isDomainErr(err) {
			return nil, err
		}
		return nil, fmt.Errorf("add books: %w", err)
	}
	for _, b := range res.AlreadyIn {
		if i, ok := firstIdx[b]; ok && reasons[i] == "" {
			reasons[i] = SkipAlreadyIn
		}
	}
	out := &BulkResult{Added: make([]AddedBook, 0, len(res.Added)), Skipped: []SkippedBook{}, BooksCount: res.Total}
	for _, a := range res.Added {
		out.Added = append(out.Added, AddedBook{BookID: a.BookID, Position: a.Position})
	}
	for i, r := range reasons {
		if r != "" {
			out.Skipped = append(out.Skipped, SkippedBook{BookID: bookIDs[i], Reason: r})
		}
	}
	return out, nil
}
