package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
)

func TestValidateResponse(t *testing.T) {
	enc := func(known bool, text string) string {
		raw, _ := json.Marshal(map[string]any{"known": known, "description": text})
		return string(raw)
	}
	tests := []struct {
		name       string
		resp       llm.StructuredResponse
		wantStatus string
		wantCode   string
	}{
		{"refusal", llm.StructuredResponse{StopReason: llm.StopReasonRefusal}, domain.StatusFailed, domain.ErrorModelRefusal},
		{"truncated", llm.StructuredResponse{StopReason: llm.StopReasonMaxTokens, JSON: []byte(`{"known":true,`)}, domain.StatusFailed, domain.ErrorModelTruncated},
		{"invalid json", answer(`{"known":true`), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"unknown field", answer(`{"known":true,"description":"` + validText + `","extra":1}`), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"missing field", answer(`{"known":true}`), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"trailing data", answer(enc(true, validText) + `{}`), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"unknown, empty", answer(enc(false, "  ")), domain.StatusUnknown, ""},
		{"unknown with text", answer(enc(false, validText)), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"too short", answer(enc(true, "Роман о любви.")), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"too long", answer(enc(true, strings.Repeat("Роман о любви. ", 80))), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"english", answer(enc(true, "The novel tells the story of a young doctor in a remote rural hospital.")), domain.StatusFailed, domain.ErrorInvalidOutput},
		{"valid", answer(enc(true, "  "+validText+"\n")), domain.StatusDraft, ""},
	}
	for _, tt := range tests {
		got := ValidateResponse(tt.resp)
		if got.Status != tt.wantStatus || got.ErrorCode != tt.wantCode {
			t.Errorf("%s: got %+v, want %s/%s", tt.name, got, tt.wantStatus, tt.wantCode)
		}
		if got.Status != domain.StatusDraft && got.Text != "" {
			t.Errorf("%s: non-draft must have no text, got %q", tt.name, got.Text)
		}
	}
	want := "Роман рассказывает о молодом враче, который приезжает в уездную больницу."
	if got := ValidateResponse(answer(enc(true, "Роман\tрассказывает  о\nмолодом враче, который приезжает в уездную больницу."))); got.Text != want {
		t.Errorf("normalised text = %q", got.Text)
	}
}

func TestValidateEditedText(t *testing.T) {
	if got, err := ValidateEditedText("  Короткий  текст. "); err != nil || got != "Короткий текст." {
		t.Errorf("valid = %q (%v)", got, err)
	}
	for i, bad := range []string{"", "   ", strings.Repeat("ж", domain.MaxEditedRunes+1), "An English text here"} {
		if _, err := ValidateEditedText(bad); err != ErrInvalidText {
			t.Errorf("case %d: err = %v", i, err)
		}
	}
}
