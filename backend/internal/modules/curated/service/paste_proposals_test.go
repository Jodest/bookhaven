package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/google/uuid"
)

func TestMatchProposals_Statuses(t *testing.T) {
	hyperion := cbook("Гиперион", pub, "Дэн Симмонс")
	childhood := cbook("Childhood's End", pub, "Артур Кларк")
	dune1, dune2 := cbook("Дюна", pub, "Фрэнк Герберт"), cbook("Дюна", pub, "Кто-то Другой")
	solaris := cbook("Солярис", pub, "Станислав Лем")
	held := cbook("Пикник на обочине", pub, "Аркадий Стругацкий, Борис Стругацкий")
	s, f := newFake(hyperion, childhood, dune1, dune2, solaris, held)
	f.held = []uuid.UUID{held.ID}

	p, err := s.MatchProposals(context.Background(), f.collection.ID, []Proposal{
		{Rank: 1, Title: "Гиперион", Author: "Дэн Симмонс"},                                        // found
		{Rank: 2, Title: "Нет такой книги", Author: "Никто", OriginalTitle: "No Such Book"},        // not found
		{Rank: 3, Title: "Конец детства", Author: "Артур Кларк", OriginalTitle: "Childhood's End"}, // rescued by the original title
		{Rank: 4, Title: "Солярис", Author: "Не Лем"},                                              // author mismatch -> ambiguous
		{Rank: 5, Title: "Пикник на обочине", Author: "Аркадий Стругацкий"},                        // in collection
		{Rank: 6, Title: "Гиперион", Author: "Дэн Симмонс"},                                        // duplicate of rank 1
		{Rank: 7, Title: "Дюна"}, // ambiguous, two candidates
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.PasteStatus{
		domain.PasteFound, domain.PasteNotFound, domain.PasteFound, domain.PasteAmbiguous,
		domain.PasteInCollection, domain.PasteDuplicate, domain.PasteAmbiguous,
	}
	for i, w := range want {
		if p.Lines[i].Status != w {
			t.Errorf("rank %d: status %s, want %s", i+1, p.Lines[i].Status, w)
		}
		if p.Lines[i].Line != i+1 {
			t.Errorf("line number %d, want the rank %d", p.Lines[i].Line, i+1)
		}
	}
	if l := p.Lines[0]; l.Text != "Гиперион — Дэн Симмонс" || l.Book == nil || l.Book.Book.ID != hyperion.ID {
		t.Errorf("rank 1: %+v", l)
	}
	if l := p.Lines[2]; l.Book == nil || l.Book.Book.ID != childhood.ID || l.Title != "Конец детства" || l.Author == nil || *l.Author != "Артур Кларк" {
		t.Errorf("rank 3 keeps the proposal title and author: %+v", l)
	}
	if l := p.Lines[3]; len(l.Candidates) != 1 || l.Candidates[0].Book.ID != solaris.ID {
		t.Errorf("rank 4: %+v", l)
	}
	if l := p.Lines[4]; l.Book == nil || !l.Book.InCollection {
		t.Errorf("rank 5: %+v", l)
	}
	if d := p.Lines[5].DuplicateOf; d == nil || *d != 1 {
		t.Errorf("rank 6 duplicate_of: %v", d)
	}
	if l := p.Lines[6]; len(l.Candidates) != 2 || l.Text != "Дюна" || l.Author != nil {
		t.Errorf("rank 7: %+v", l)
	}
	if p.Counts[domain.PasteFound] != 2 || p.Counts[domain.PasteNotFound] != 1 || p.Counts[domain.PasteInvalid] != 0 || p.BooksCount != 1 {
		t.Errorf("counts: %+v", p)
	}
}

func TestMatchProposals_OriginalTitleKeepsTheAuthorCheck(t *testing.T) {
	// The catalog holds «Childhood's End» by someone else: the retry keeps
	// the proposal author, so it is not a confident match.
	other := cbook("Childhood's End", pub, "Другой Автор")
	s, f := newFake(other)
	p, err := s.MatchProposals(context.Background(), f.collection.ID, []Proposal{
		{Rank: 1, Title: "Конец детства", Author: "Артур Кларк", OriginalTitle: "Childhood's End"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Lines[0].Status != domain.PasteAmbiguous || len(p.Lines[0].Candidates) != 1 {
		t.Errorf("author mismatch through the original title: %+v", p.Lines[0])
	}
}

func TestMatchProposals_RetryOnlyWhenTitleMissesAndKeyDiffers(t *testing.T) {
	hyperion := cbook("Гиперион", pub, "Дэн Симмонс")
	s, f := newFake(hyperion)
	_, err := s.MatchProposals(context.Background(), f.collection.ID, []Proposal{
		{Rank: 1, Title: "Гиперион", Author: "Дэн Симмонс", OriginalTitle: "Hyperion"}, // matches by title: no retry
		{Rank: 2, Title: "Нету", OriginalTitle: "нету"},                                // same key: no retry
		{Rank: 3, Title: "Нету-2", Author: "Автор", OriginalTitle: "Hyperion"},         // retry by the original title
	})
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, q := range f.queries {
		count[q]++
	}
	if count["hyperion"] != 1 {
		t.Errorf("original-title queries = %d (%v), want 1", count["hyperion"], f.queries)
	}
}

func TestMatchProposals_CollectionErrors(t *testing.T) {
	s, f := newFake()
	f.collection.Kind = domain.KindAuto
	if _, err := s.MatchProposals(context.Background(), f.collection.ID, nil); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("automatic: %v", err)
	}
	if _, err := s.MatchProposals(context.Background(), uuid.New(), nil); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestMatchProposals_WritesNothing(t *testing.T) {
	s, f := newFake(cbook("Гиперион", pub, "Дэн Симмонс"))
	if _, err := s.MatchProposals(context.Background(), f.collection.ID, []Proposal{{Rank: 1, Title: "Гиперион"}}); err != nil {
		t.Fatal(err)
	}
	if f.addCalls != 0 {
		t.Error("matching must never add a book")
	}
}

func TestManualInfo(t *testing.T) {
	s, f := newFake()
	f.collection.Title, f.collection.Description, f.collection.Size = "Фантастика", "про космос", 50
	info, err := s.ManualInfo(context.Background(), f.collection.ID)
	if err != nil || info.Title != "Фантастика" || info.Description != "про космос" || info.Size != 50 || info.GenreName != "" {
		t.Fatalf("info = %+v (%v)", info, err)
	}
	f.collection.Kind = domain.KindAuto
	if _, err := s.ManualInfo(context.Background(), f.collection.ID); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Errorf("automatic: %v", err)
	}
	if _, err := s.ManualInfo(context.Background(), uuid.New()); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Errorf("unknown: %v", err)
	}
}
