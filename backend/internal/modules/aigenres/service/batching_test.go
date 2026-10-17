package service

import (
	"fmt"
	"testing"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func cand(name, status string, books int64, created int) candidate {
	return candidate{
		Genre: catalogDomain.Genre{
			ID: uuid.New(), Name: name, Status: status, CreatedAt: now0.Add(time.Duration(created) * time.Minute),
		},
		BooksTotal: books,
	}
}

func ids(cs []candidate) map[uuid.UUID]bool {
	m := map[uuid.UUID]bool{}
	for _, c := range cs {
		m[c.Genre.ID] = true
	}
	return m
}

func TestPlanBatches_AnchorsAndSizes(t *testing.T) {
	var cands []candidate
	for i := 0; i < 150; i++ {
		cands = append(cands, cand(fmt.Sprintf("pub %03d", i), "published", int64(1000-i), i))
	}
	for i := 0; i < 1200; i++ {
		cands = append(cands, cand(fmt.Sprintf("pend %04d", i), "pending", 0, 1000+i))
	}
	plan := planBatches(cands, nil, 300, 10)
	if len(plan.Anchors) != 100 {
		t.Fatalf("anchors = %d", len(plan.Anchors))
	}
	if plan.Anchors[0].BooksTotal != 1000 || plan.Anchors[99].BooksTotal != 901 {
		t.Fatalf("anchors must be the most popular published ones: %d..%d", plan.Anchors[0].BooksTotal, plan.Anchors[99].BooksTotal)
	}
	// 50 published non-anchors + 1200 pending = 1250 -> 4 full batches + 1.
	if len(plan.Batches) != 5 {
		t.Fatalf("batches = %d", len(plan.Batches))
	}
	total := 0
	anchorSet := ids(plan.Anchors)
	for i, b := range plan.Batches {
		total += len(b)
		for _, c := range b {
			if anchorSet[c.Genre.ID] {
				t.Fatalf("anchor repeated inside a batch")
			}
		}
		if i < 4 && len(b) != 300 {
			t.Fatalf("batch %d has %d genres", i, len(b))
		}
	}
	if total != 1250 {
		t.Fatalf("every non-anchor must be planned once, got %d", total)
	}
	// Pending first (oldest first), then published by the fewest books.
	if plan.Batches[0][0].Genre.Status != "pending" || plan.Batches[0][0].Genre.Name != "pend 0000" {
		t.Fatalf("first genre: %+v", plan.Batches[0][0].Genre)
	}
	last := plan.Batches[4]
	if len(last) != 50 || last[len(last)-1].Genre.Status != "published" {
		t.Fatalf("published genres must come last: %d %+v", len(last), last[len(last)-1].Genre)
	}
}

func TestPlanBatches_PublishedOrderByFewestBooks(t *testing.T) {
	var cands []candidate
	for i := 0; i < 103; i++ {
		cands = append(cands, cand(fmt.Sprintf("g%03d", i), "published", int64(10+i), i))
	}
	plan := planBatches(cands, nil, 10, 10)
	if len(plan.Batches) != 1 || len(plan.Batches[0]) != 3 {
		t.Fatalf("batches: %d", len(plan.Batches))
	}
	for i, c := range plan.Batches[0] {
		if c.BooksTotal != int64(10+i) {
			t.Fatalf("expected ascending books, got %d at %d", c.BooksTotal, i)
		}
	}
}

func TestPlanBatches_HintedGroupsStayTogether(t *testing.T) {
	var cands []candidate
	for i := 0; i < 20; i++ {
		cands = append(cands, cand(fmt.Sprintf("solo %02d", i), "pending", 0, i))
	}
	// Three hinted genres created far apart: the group lands where its first
	// member lands (the very first batch) even though size is 5.
	a := cand("романы", "pending", 0, 100)
	b := cand("роман", "pending", 0, 5)
	c := cand("романа", "pending", 0, 150)
	cands = append(cands, a, b, c)
	pairs := []adminservice.GenreHintPair{
		{A: a.Genre.ID, B: b.Genre.ID, Match: adminservice.GenreMatchSameStem},
		{A: b.Genre.ID, B: c.Genre.ID, Match: adminservice.GenreMatchSameStem},
	}
	plan := planBatches(cands, pairs, 5, 10)
	where := map[uuid.UUID]int{}
	for i, batch := range plan.Batches {
		for _, x := range batch {
			where[x.Genre.ID] = i
		}
	}
	if where[a.Genre.ID] != where[b.Genre.ID] || where[b.Genre.ID] != where[c.Genre.ID] {
		t.Fatalf("hinted genres split: %v", where)
	}
	if len(where) != 23 {
		t.Fatalf("every genre must be planned once, got %d", len(where))
	}
	// The group sits in the batch of its first member (b, the 6th oldest).
	if where[b.Genre.ID] != 1 {
		t.Fatalf("group batch = %d", where[b.Genre.ID])
	}
}

func TestPlanBatches_CapsAtMaxBatchesAndOversizedGroup(t *testing.T) {
	var cands []candidate
	for i := 0; i < 500; i++ {
		cands = append(cands, cand(fmt.Sprintf("p%03d", i), "pending", 0, i))
	}
	plan := planBatches(cands, nil, 10, 10)
	if len(plan.Batches) != 10 {
		t.Fatalf("batches = %d, want 10", len(plan.Batches))
	}

	// One group of 80 genres is cut at 50.
	cands = cands[:80]
	var pairs []adminservice.GenreHintPair
	for i := 1; i < len(cands); i++ {
		pairs = append(pairs, adminservice.GenreHintPair{A: cands[i-1].Genre.ID, B: cands[i].Genre.ID, Match: adminservice.GenreMatchSharedWord})
	}
	plan = planBatches(cands, pairs, 10, 10)
	seen := map[uuid.UUID]int{}
	for _, b := range plan.Batches {
		for _, x := range b {
			seen[x.Genre.ID]++
		}
	}
	if len(seen) != 80 {
		t.Fatalf("planned %d of 80", len(seen))
	}
	for _, n := range seen {
		if n != 1 {
			t.Fatal("a genre was planned twice")
		}
	}
	if len(plan.Batches[0]) != 50 {
		t.Fatalf("first batch = %d, want the capped group of 50", len(plan.Batches[0]))
	}
}

func TestPlanBatches_OnlyAnchorsAndEmpty(t *testing.T) {
	var cands []candidate
	for i := 0; i < 5; i++ {
		cands = append(cands, cand(fmt.Sprintf("p%d", i), "published", int64(i), i))
	}
	plan := planBatches(cands, nil, 300, 10)
	if len(plan.Anchors) != 5 || len(plan.Batches) != 1 || len(plan.Batches[0]) != 0 {
		t.Fatalf("anchors-only catalog: %d anchors, %d batches", len(plan.Anchors), len(plan.Batches))
	}
	if empty := planBatches(nil, nil, 300, 10); len(empty.Batches) != 0 {
		t.Fatalf("empty: %+v", empty)
	}
}

func TestFingerprint_ChangesWithNameStatusQIDAndMembers(t *testing.T) {
	a := cand("Фэнтези", "published", 1, 1)
	base := Fingerprint([]candidate{a})
	if Fingerprint([]candidate{a}) != base {
		t.Fatal("fingerprint must be stable")
	}
	renamed := a
	renamed.Genre.Name = "Фентези"
	statused := a
	statused.Genre.Status = "pending"
	qid := "Q1"
	withQID := a
	withQID.Genre.WikidataQID = &qid
	books := a
	books.BooksTotal = 99
	for name, c := range map[string]candidate{"name": renamed, "status": statused, "qid": withQID} {
		if Fingerprint([]candidate{c}) == base {
			t.Errorf("%s change must change the fingerprint", name)
		}
	}
	if Fingerprint([]candidate{books}) != base {
		t.Error("a changed book count alone must not change the fingerprint")
	}
	if Fingerprint([]candidate{a, cand("Другой", "pending", 0, 2)}) == base {
		t.Error("a new genre must change the fingerprint")
	}
}
