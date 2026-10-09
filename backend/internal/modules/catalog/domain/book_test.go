package domain

import "testing"

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
