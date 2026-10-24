package http

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/google/uuid"
)

// In-memory reading journal mirroring GormRepository semantics.

func (f *fakeRepository) openIdx(userID, bookID uuid.UUID) int {
	for i, r := range f.reads {
		if r.UserID == userID && r.BookID == bookID && r.FinishedAt == nil && r.AbandonedAt == nil {
			return i
		}
	}
	return -1
}

func (f *fakeRepository) OpenRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error {
	if f.openIdx(userID, bookID) >= 0 {
		return nil
	}
	f.reads = append(f.reads, domain.BookRead{ID: uuid.New(), UserID: userID, BookID: bookID, StartedAt: &now, CreatedAt: now})
	return nil
}

func (f *fakeRepository) FinishRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error {
	if i := f.openIdx(userID, bookID); i >= 0 {
		f.reads[i].FinishedAt = &now
		return nil
	}
	f.reads = append(f.reads, domain.BookRead{ID: uuid.New(), UserID: userID, BookID: bookID, FinishedAt: &now, CreatedAt: now})
	return nil
}

func (f *fakeRepository) AbandonOpenRead(ctx context.Context, userID, bookID uuid.UUID, now time.Time) error {
	if i := f.openIdx(userID, bookID); i >= 0 {
		f.reads[i].AbandonedAt = &now
	}
	return nil
}

func (f *fakeRepository) DeleteOpenRead(ctx context.Context, userID, bookID uuid.UUID) error {
	if i := f.openIdx(userID, bookID); i >= 0 {
		f.reads = append(f.reads[:i], f.reads[i+1:]...)
	}
	return nil
}

func (f *fakeRepository) DeleteLatestFinishedRead(ctx context.Context, userID, bookID uuid.UUID) error {
	latest := -1
	for i, r := range f.reads {
		if r.UserID != userID || r.BookID != bookID || r.FinishedAt == nil {
			continue
		}
		if latest < 0 || r.FinishedAt.After(*f.reads[latest].FinishedAt) ||
			(r.FinishedAt.Equal(*f.reads[latest].FinishedAt) && r.CreatedAt.After(f.reads[latest].CreatedAt)) {
			latest = i
		}
	}
	if latest >= 0 {
		f.reads = append(f.reads[:latest], f.reads[latest+1:]...)
	}
	return nil
}

func (f *fakeRepository) ReadSummary(ctx context.Context, userID, bookID uuid.UUID) (domain.ReadSummary, error) {
	counts, _ := f.ReadCounts(ctx, userID, []uuid.UUID{bookID})
	c := counts[bookID]
	return domain.ReadSummary{Count: c.Count, LastReadAt: c.LastReadAt, Rereading: c.Count > 0 && f.openIdx(userID, bookID) >= 0}, nil
}

func (f *fakeRepository) ReadCounts(ctx context.Context, userID uuid.UUID, bookIDs []uuid.UUID) (map[uuid.UUID]domain.ReadCount, error) {
	want := map[uuid.UUID]bool{}
	for _, id := range bookIDs {
		want[id] = true
	}
	out := map[uuid.UUID]domain.ReadCount{}
	for _, r := range f.reads {
		if r.UserID != userID || !want[r.BookID] || r.FinishedAt == nil {
			continue
		}
		c := out[r.BookID]
		c.Count++
		if c.LastReadAt == nil || r.FinishedAt.After(*c.LastReadAt) {
			t := *r.FinishedAt
			c.LastReadAt = &t
		}
		out[r.BookID] = c
	}
	return out, nil
}
