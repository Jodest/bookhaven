package repository

import (
	"testing"

	"github.com/google/uuid"
)

// fakeNotificationsTable re-implements, for unit testing, the
// `UNIQUE (actor_id, review_id, type)` upsert semantics of
// GormRepository.upsertNotificationInTx's
// `INSERT ... ON CONFLICT (actor_id, review_id, type) DO UPDATE SET
// is_read = false, created_at = NOW()`, following this codebase's
// hand-rolled-fake convention for repository-level logic tests (see
// catalog/repository/repository_test.go, admin/repository/repository_test.go)
// since there is no real-Postgres integration test harness here.
type notificationKey struct {
	actorID  uuid.UUID
	reviewID uuid.UUID
	typ      string
}

type fakeNotificationRow struct {
	isRead    bool
	createdAt int // logical clock - only relative ordering matters here
}

type fakeNotificationsTable struct {
	rows  map[notificationKey]*fakeNotificationRow
	clock int
}

func newFakeNotificationsTable() *fakeNotificationsTable {
	return &fakeNotificationsTable{rows: make(map[notificationKey]*fakeNotificationRow)}
}

// upsert mirrors upsertNotificationInTx's INSERT ... ON CONFLICT DO UPDATE.
func (t *fakeNotificationsTable) upsert(actorID, reviewID uuid.UUID, typ string) {
	t.clock++
	key := notificationKey{actorID: actorID, reviewID: reviewID, typ: typ}
	if row, ok := t.rows[key]; ok {
		row.isRead = false
		row.createdAt = t.clock
		return
	}
	t.rows[key] = &fakeNotificationRow{isRead: false, createdAt: t.clock}
}

func TestNotificationUpsert_FirstLikeCreatesUnreadRow(t *testing.T) {
	table := newFakeNotificationsTable()
	actorID := uuid.New()
	reviewID := uuid.New()

	table.upsert(actorID, reviewID, "review_like")

	if len(table.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(table.rows))
	}
	key := notificationKey{actorID: actorID, reviewID: reviewID, typ: "review_like"}
	if table.rows[key].isRead {
		t.Fatal("expected a freshly created notification to be unread")
	}
}

func TestNotificationUpsert_RepeatedLikeRefreshesSingleRow(t *testing.T) {
	table := newFakeNotificationsTable()
	actorID := uuid.New()
	reviewID := uuid.New()
	key := notificationKey{actorID: actorID, reviewID: reviewID, typ: "review_like"}

	table.upsert(actorID, reviewID, "review_like")
	table.rows[key].isRead = true // simulate the recipient having read it
	firstCreatedAt := table.rows[key].createdAt

	// A like/unlike/like cycle re-runs the same upsert (unliking does not
	// delete the row - only a renewed like re-upserts it).
	table.upsert(actorID, reviewID, "review_like")

	if len(table.rows) != 1 {
		t.Fatalf("expected the row to be reused, not duplicated; got %d rows", len(table.rows))
	}
	if table.rows[key].isRead {
		t.Fatal("expected the row to be refreshed back to unread")
	}
	if table.rows[key].createdAt <= firstCreatedAt {
		t.Fatal("expected created_at to be refreshed to a later value")
	}
}

func TestNotificationUpsert_DifferentActorsOnSameReviewGetSeparateRows(t *testing.T) {
	table := newFakeNotificationsTable()
	reviewID := uuid.New()
	actorA := uuid.New()
	actorB := uuid.New()

	table.upsert(actorA, reviewID, "review_like")
	table.upsert(actorB, reviewID, "review_like")

	if len(table.rows) != 2 {
		t.Fatalf("expected 2 separate rows for 2 distinct actors, got %d", len(table.rows))
	}
}
