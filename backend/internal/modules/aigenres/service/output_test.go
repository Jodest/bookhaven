package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
)

func testRefs() map[string]refGenre {
	return map[string]refGenre{
		"g1": {ID: uuid.New(), Name: "Фэнтези", Books: 120},
		"g2": {ID: uuid.New(), Name: "Фентези", Books: 3},
		"g3": {ID: uuid.New(), Name: "Fantasy", Books: 2},
		"g4": {ID: uuid.New(), Name: "Киберпанк", Books: 30},
		"g5": {ID: uuid.New(), Name: "Киберпанк-нуар", Books: 40},
		"g6": {ID: uuid.New(), Name: "Книги 2020", Books: 5},
		"g7": {ID: uuid.New(), Name: "Нуар", Books: 4},
	}
}

func raw(v any) llm.StructuredResponse {
	return llm.StructuredResponse{JSON: jsonResp(v).resp.JSON, StopReason: llm.StopReasonEndTurn}
}

func TestValidate_StoresDuplicateAndJunk(t *testing.T) {
	refs := testRefs()
	out := Validate(raw(answer(
		dup("g1", []string{"g2", "g3"}, 0.954),
		junkItem("g6", 0.8),
	)), refs, "m")
	if out.Code != "" || len(out.Items) != 2 || out.Dropped != 0 {
		t.Fatalf("outcome: %+v", out)
	}
	d := out.Items[0]
	if d.Kind != domain.KindDuplicate || len(d.Genres) != 3 || d.Genres[0].Role != domain.RoleTarget ||
		d.Genres[0].GenreID != refs["g1"].ID || d.Genres[1].GenreID != refs["g2"].ID || d.Genres[2].NameSnapshot != "Fantasy" {
		t.Fatalf("duplicate genres: %+v", d.Genres)
	}
	if d.Confidence != 0.95 || d.Model != "m" || d.PromptVersion != PromptVersion || d.Reason != "одно и то же" {
		t.Fatalf("duplicate fields: %+v", d)
	}
	j := out.Items[1]
	if j.Kind != domain.KindJunk || len(j.Genres) != 1 || j.Genres[0].Role != domain.RoleSource || j.Genres[0].GenreID != refs["g6"].ID {
		t.Fatalf("junk: %+v", j)
	}
}

func TestValidate_FixedOutcomes(t *testing.T) {
	refs := testRefs()
	cases := []struct {
		name string
		resp llm.StructuredResponse
		code string
	}{
		{"refusal", llm.StructuredResponse{StopReason: llm.StopReasonRefusal}, domain.ErrorModelRefusal},
		{"truncated", llm.StructuredResponse{StopReason: llm.StopReasonMaxTokens, JSON: []byte(`{"sugg`)}, domain.ErrorModelTruncated},
		{"not json", llm.StructuredResponse{StopReason: "end_turn", JSON: []byte(`nope`)}, domain.ErrorInvalidOutput},
		{"trailing data", llm.StructuredResponse{StopReason: "end_turn", JSON: []byte(`{"suggestions":[]} {}`)}, domain.ErrorInvalidOutput},
		{"no list", llm.StructuredResponse{StopReason: "end_turn", JSON: []byte(`{}`)}, domain.ErrorInvalidOutput},
		{"unknown field", llm.StructuredResponse{StopReason: "end_turn", JSON: []byte(`{"suggestions":[],"x":1}`)}, domain.ErrorInvalidOutput},
	}
	for _, tc := range cases {
		out := Validate(tc.resp, refs, "m")
		if out.Code != tc.code || len(out.Items) != 0 {
			t.Errorf("%s: got %+v, want code %s", tc.name, out, tc.code)
		}
	}
	if out := Validate(raw(answer()), refs, "m"); out.Code != "" || len(out.Items) != 0 {
		t.Errorf("empty list must be fine: %+v", out)
	}
}

func TestValidate_DropRules(t *testing.T) {
	refs := testRefs()
	narrow := func(target string, genres []string, conf float64) item {
		return item{"kind": "narrow", "target": target, "genres": genres, "reason_codes": []string{"too_narrow"}, "reason": "r", "confidence": conf}
	}
	manyRefs := map[string]refGenre{"t": {ID: uuid.New(), Name: "T", Books: 100}}
	var many []string
	for i := 0; i < 20; i++ {
		ref := fmt.Sprintf("s%d", i)
		manyRefs[ref] = refGenre{ID: uuid.New(), Name: ref, Books: 1}
		many = append(many, ref)
	}

	cases := []struct {
		name string
		refs map[string]refGenre
		it   item
	}{
		{"unknown kind", refs, item{"kind": "other", "target": "g1", "genres": []string{"g2"}, "reason_codes": []string{"spelling"}, "reason": "r", "confidence": 0.9}},
		{"unknown code", refs, item{"kind": "duplicate", "target": "g1", "genres": []string{"g2"}, "reason_codes": []string{"weird"}, "reason": "r", "confidence": 0.9}},
		{"code of another kind", refs, item{"kind": "duplicate", "target": "g1", "genres": []string{"g2"}, "reason_codes": []string{"too_narrow"}, "reason": "r", "confidence": 0.9}},
		{"no codes", refs, item{"kind": "duplicate", "target": "g1", "genres": []string{"g2"}, "reason_codes": []string{}, "reason": "r", "confidence": 0.9}},
		{"unknown source ref", refs, dup("g1", []string{"g99"}, 0.9)},
		{"unknown target ref", refs, dup("g99", []string{"g2"}, 0.9)},
		{"genre twice", refs, dup("g1", []string{"g2", "g2"}, 0.9)},
		{"no target", refs, item{"kind": "duplicate", "target": nil, "genres": []string{"g2"}, "reason_codes": []string{"spelling"}, "reason": "r", "confidence": 0.9}},
		{"no source", refs, dup("g1", []string{}, 0.9)},
		{"target among sources", refs, dup("g1", []string{"g1", "g2"}, 0.9)},
		{"more than 20 genres", manyRefs, dup("t", many, 0.9)},
		{"junk with a target", refs, item{"kind": "junk", "target": "g1", "genres": []string{"g6"}, "reason_codes": []string{"date"}, "reason": "r", "confidence": 0.9}},
		{"junk with two genres", refs, item{"kind": "junk", "target": nil, "genres": []string{"g6", "g2"}, "reason_codes": []string{"date"}, "reason": "r", "confidence": 0.9}},
		{"narrow source with many books", refs, narrow("g4", []string{"g5"}, 0.9)},
		{"narrow source not smaller than target", refs, narrow("g3", []string{"g2"}, 0.9)},
		{"confidence above 1", refs, dup("g1", []string{"g2"}, 1.5)},
		{"confidence negative", refs, dup("g1", []string{"g2"}, -0.1)},
		{"confidence below 0.5", refs, dup("g1", []string{"g2"}, 0.49)},
		{"missing reason", refs, item{"kind": "duplicate", "target": "g1", "genres": []string{"g2"}, "reason_codes": []string{"spelling"}, "confidence": 0.9}},
	}
	for _, tc := range cases {
		out := Validate(raw(answer(tc.it)), tc.refs, "m")
		if out.Code != "" || len(out.Items) != 0 || out.Dropped != 1 {
			t.Errorf("%s: expected one dropped item, got %+v", tc.name, out)
		}
	}
}

func TestValidate_NarrowAccepted(t *testing.T) {
	refs := testRefs()
	out := Validate(raw(answer(item{
		"kind": "narrow", "target": "g4", "genres": []string{"g7"}, "reason_codes": []string{"too_narrow", "too_narrow"}, "reason": "r", "confidence": 0.7,
	})), refs, "m")
	if len(out.Items) != 1 || len(out.Items[0].ReasonCodes) != 1 {
		t.Fatalf("narrow: %+v", out)
	}
}

func TestValidate_GenreAlreadyUsedByEarlierItem(t *testing.T) {
	refs := testRefs()
	out := Validate(raw(answer(
		dup("g1", []string{"g2"}, 0.9),
		dup("g4", []string{"g2"}, 0.9), // g2 already used
		junkItem("g1", 0.9),            // g1 (a target) already used
		junkItem("g6", 0.9),            // fine
		dup("g6", []string{"g3"}, 0.9), // g6 used by the junk item
	)), refs, "m")
	if len(out.Items) != 2 || out.Dropped != 3 {
		t.Fatalf("expected 2 kept and 3 dropped, got %d kept, %d dropped", len(out.Items), out.Dropped)
	}
}

func TestValidate_CapsItemsAndTruncatesReason(t *testing.T) {
	refs := map[string]refGenre{}
	var items []item
	for i := 0; i < 60; i++ {
		ref := fmt.Sprintf("g%d", i+1)
		refs[ref] = refGenre{ID: uuid.New(), Name: ref, Books: 1}
		it := junkItem(ref, 0.9)
		it["reason"] = "  " + strings.Repeat("я", 400)
		items = append(items, it)
	}
	out := Validate(raw(answer(items...)), refs, "m")
	if len(out.Items) != domain.MaxItemsPerAnswer {
		t.Fatalf("expected %d items, got %d", domain.MaxItemsPerAnswer, len(out.Items))
	}
	if n := len([]rune(out.Items[0].Reason)); n != domain.MaxReasonRunes {
		t.Fatalf("reason runes = %d", n)
	}
}
