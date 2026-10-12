package domain

import (
	"testing"
	"time"
)

func TestIsValidBookStatus(t *testing.T) {
	for _, s := range []string{BookStatusPending, BookStatusPublished, BookStatusRejected} {
		if !IsValidBookStatus(s) {
			t.Fatalf("%q must be valid", s)
		}
	}
	for _, s := range []string{"", "hidden", "visible", "Published", "pending_review"} {
		if IsValidBookStatus(s) {
			t.Fatalf("%q must be invalid", s)
		}
	}
}

func TestBookIsPublished(t *testing.T) {
	if !(Book{Status: BookStatusPublished}).IsPublished() {
		t.Fatal("published book must be published")
	}
	for _, s := range []string{BookStatusPending, BookStatusRejected, ""} {
		if (Book{Status: s}).IsPublished() {
			t.Fatalf("%q book must not be published", s)
		}
	}
}

func TestBookMarkPublished(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

	pending := Book{Status: BookStatusPending}
	pending.MarkPublished(now)
	if pending.PublishedAt != nil {
		t.Fatalf("pending book must keep a nil PublishedAt, got %v", pending.PublishedAt)
	}

	fresh := Book{Status: BookStatusPublished}
	fresh.MarkPublished(now)
	if fresh.PublishedAt == nil || !fresh.PublishedAt.Equal(now) {
		t.Fatalf("published book without a time must get now, got %v", fresh.PublishedAt)
	}

	earlier := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	republished := Book{Status: BookStatusPublished, PublishedAt: &earlier}
	republished.MarkPublished(now)
	if republished.PublishedAt == nil || !republished.PublishedAt.Equal(earlier) {
		t.Fatalf("existing PublishedAt must be kept, got %v", republished.PublishedAt)
	}
}
