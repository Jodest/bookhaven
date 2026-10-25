package service

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

func (f *fakeRepository) DeleteLatestFinishedRead(ctx context.Context, userID, bookID uuid.UUID, since time.Time) error {
	latest := -1
	for i, r := range f.reads {
		if r.UserID != userID || r.BookID != bookID || r.FinishedAt == nil || r.ManualEnd || r.FinishedAt.Before(since) {
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

func (f *fakeRepository) ListReads(ctx context.Context, userID, bookID uuid.UUID) ([]domain.BookRead, error) {
	var out []domain.BookRead
	for _, r := range f.reads {
		if r.UserID == userID && r.BookID == bookID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepository) GetRead(ctx context.Context, userID, readID uuid.UUID) (*domain.BookRead, error) {
	for _, r := range f.reads {
		if r.ID == readID && r.UserID == userID {
			c := r
			return &c, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) CountReads(ctx context.Context, userID, bookID uuid.UUID) (int64, error) {
	var n int64
	for _, r := range f.reads {
		if r.UserID == userID && r.BookID == bookID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepository) InsertFinishedRead(ctx context.Context, read *domain.BookRead) error {
	f.reads = append(f.reads, *read)
	return nil
}

func (f *fakeRepository) UpdateReadDates(ctx context.Context, userID, readID uuid.UUID, loadedState domain.ReadState, fields map[string]any) (int64, error) {
	for i := range f.reads {
		r := &f.reads[i]
		if r.ID != readID || r.UserID != userID || r.State() != loadedState {
			continue
		}
		for k, v := range fields {
			var t *time.Time
			if tv, ok := v.(time.Time); ok {
				t = &tv
			}
			switch k {
			case "started_at":
				r.StartedAt = t
			case "finished_at":
				r.FinishedAt = t
			case "abandoned_at":
				r.AbandonedAt = t
			case "manual_end":
				r.ManualEnd = v.(bool)
			}
		}
		return 1, nil
	}
	return 0, nil
}

func (f *fakeRepository) DeleteClosedRead(ctx context.Context, userID, readID uuid.UUID) (int64, error) {
	for i, r := range f.reads {
		if r.ID == readID && r.UserID == userID && r.State() != domain.ReadStateOpen {
			f.reads = append(f.reads[:i], f.reads[i+1:]...)
			return 1, nil
		}
	}
	return 0, nil
}

func (f *fakeRepository) ShelfItemAddedAt(ctx context.Context, shelfID, bookID uuid.UUID) (time.Time, bool, error) {
	at, ok := f.items[shelfBookKey{shelfID: shelfID, bookID: bookID}]
	return at, ok, nil
}
