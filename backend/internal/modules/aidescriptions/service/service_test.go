package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func TestService_StatusAndList(t *testing.T) {
	fresh := book(catalogDomain.BookStatusPublished, nil)
	edited := book(catalogDomain.BookStatusPublished, nil)
	oldPrompt := book(catalogDomain.BookStatusPublished, nil)
	repo := newFakeRepo()
	repo.missing = []uuid.UUID{uuid.New(), uuid.New()}
	item := func(b catalogDomain.Book, version int, hash string) domain.Item {
		return domain.Item{Description: domain.Description{BookID: b.ID, Status: domain.StatusDraft, PromptVersion: version, InputHash: hash}}
	}
	changed := edited
	changed.Title = "Старое название"
	repo.listItems = []domain.Item{
		item(fresh, PromptVersion, InputHash(InputOf(fresh))),
		item(edited, PromptVersion, InputHash(InputOf(changed))),
		item(oldPrompt, PromptVersion+1, InputHash(InputOf(oldPrompt))),
	}
	books := booksOf(fresh, edited, oldPrompt)
	svc := NewService(repo, books, true, "claude-sonnet-5-5")

	st, err := svc.Status(context.Background())
	if err != nil || !st.Enabled || st.Model != "claude-sonnet-5-5" || st.Missing != 2 || st.Drafts != 6 {
		t.Fatalf("status = %+v (%v)", st, err)
	}

	res, err := svc.List(context.Background(), "", 0, 500)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Page != 1 || res.Limit != domain.MaxListLimit || repo.listArgs[0] != domain.StatusDraft {
		t.Errorf("paging = %d/%d args=%v", res.Page, res.Limit, repo.listArgs)
	}
	if books.calls != 1 {
		t.Errorf("one loader call per page, got %d", books.calls)
	}
	if res.Items[0].Stale || !res.Items[1].Stale || !res.Items[2].Stale {
		t.Errorf("stale = %v %v %v", res.Items[0].Stale, res.Items[1].Stale, res.Items[2].Stale)
	}
	if len(res.Items[0].Book.Authors) != 2 || res.Items[0].Book.Authors[0] != "Аноним" {
		t.Errorf("authors = %v", res.Items[0].Book.Authors)
	}

	if _, err := svc.List(context.Background(), "bogus", 1, 20); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("unknown status: %v", err)
	}
}

func TestService_ApproveValidatesText(t *testing.T) {
	b := book(catalogDomain.BookStatusPublished, nil)
	repo := newFakeRepo()
	repo.records[b.ID] = domain.Item{Description: domain.Description{BookID: b.ID, Status: domain.StatusDraft, Text: validText}}
	svc := NewService(repo, booksOf(b), false, "m")

	if _, err := svc.Approve(context.Background(), b.ID, uuid.New(), strp("English only")); !errors.Is(err, ErrInvalidText) {
		t.Fatalf("bad text: %v", err)
	}
	if repo.approveText != nil {
		t.Error("an invalid text must not reach the repository")
	}
	// Works while the feature is off.
	it, err := svc.Approve(context.Background(), b.ID, uuid.New(), strp("  Исправленный   текст. "))
	if err != nil || it.Status != domain.StatusApproved {
		t.Fatalf("approve: %+v (%v)", it, err)
	}
	if repo.approveText == nil || *repo.approveText != "Исправленный текст." {
		t.Errorf("text = %v", repo.approveText)
	}

	repo.approveErr = ErrHasOwnDescription
	if _, err := svc.Approve(context.Background(), b.ID, uuid.New(), nil); !errors.Is(err, ErrHasOwnDescription) {
		t.Errorf("own description: %v", err)
	}
	repo.discardErr = ErrNotFound
	if _, err := svc.Discard(context.Background(), b.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("discard: %v", err)
	}
}

func TestService_GetNone(t *testing.T) {
	svc := NewService(newFakeRepo(), booksOf(), true, "m")
	if it, err := svc.Get(context.Background(), uuid.New()); it != nil || err != nil {
		t.Errorf("get none = %+v (%v)", it, err)
	}
}
