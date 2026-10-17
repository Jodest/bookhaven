package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestSetKey_OrderIndependentAndActionSpecific(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	if SetKey("merge", []uuid.UUID{a, b, c}) != SetKey("merge", []uuid.UUID{c, a, b}) {
		t.Fatal("the key must not depend on the order (or on which genre is the target)")
	}
	if SetKey("merge", []uuid.UUID{a, b}) == SetKey("reject", []uuid.UUID{a, b}) {
		t.Fatal("the key must depend on the action")
	}
	if SetKey("merge", []uuid.UUID{a, b}) == SetKey("merge", []uuid.UUID{a, b, c}) {
		t.Fatal("a different set must give a different key")
	}
	if len(SetKey("merge", []uuid.UUID{a})) != 64 {
		t.Fatal("the key must be 64 hex chars")
	}
}

func TestReasonCodesPerKind(t *testing.T) {
	if !IsReasonCodeOf(KindDuplicate, ReasonLanguage) || IsReasonCodeOf(KindDuplicate, ReasonTooNarrow) {
		t.Fatal("duplicate codes")
	}
	if !IsReasonCodeOf(KindNarrow, ReasonTooNarrow) || IsReasonCodeOf(KindNarrow, ReasonSpelling) {
		t.Fatal("narrow codes")
	}
	if !IsReasonCodeOf(KindJunk, ReasonDate) || IsReasonCodeOf(KindJunk, ReasonSynonym) {
		t.Fatal("junk codes")
	}
	if Action(KindDuplicate) != "merge" || Action(KindNarrow) != "merge" || Action(KindJunk) != "reject" {
		t.Fatal("actions")
	}
}
