package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
)

func TestParseOutput_Failures(t *testing.T) {
	tests := []struct {
		name string
		resp llm.StructuredResponse
		want string
	}{
		{"refusal", llm.StructuredResponse{StopReason: llm.StopReasonRefusal}, domain.ErrorModelRefusal},
		{"max tokens", llm.StructuredResponse{StopReason: llm.StopReasonMaxTokens, JSON: []byte(`{"authors":[`)}, domain.ErrorModelTruncated},
		{"unknown top-level field", answer(`{"authors":[],"extra":1}`), domain.ErrorInvalidOutput},
		{"unknown item field", answer(`{"authors":[{"name":"А","note":"","qid":"Q1"}]}`), domain.ErrorInvalidOutput},
		{"missing authors", answer(`{}`), domain.ErrorInvalidOutput},
		{"missing note", answer(`{"authors":[{"name":"А"}]}`), domain.ErrorInvalidOutput},
		{"missing name", answer(`{"authors":[{"note":"x"}]}`), domain.ErrorInvalidOutput},
		{"not json", answer(`nope`), domain.ErrorInvalidOutput},
		{"trailing data", answer(`{"authors":[]} {}`), domain.ErrorInvalidOutput},
		{"empty", llm.StructuredResponse{StopReason: llm.StopReasonEndTurn}, domain.ErrorInvalidOutput},
	}
	for _, tt := range tests {
		cands, code := ParseOutput(tt.resp, 20)
		if code != tt.want || cands != nil {
			t.Errorf("%s: code=%q cands=%v, want %q", tt.name, code, cands, tt.want)
		}
	}
}

func TestParseOutput_Cleaning(t *testing.T) {
	long := strings.Repeat("я", 151)
	exact := strings.Repeat("ю", 150)
	longNote := strings.Repeat("н", 250)
	raw, _ := json.Marshal(map[string]any{"authors": []map[string]string{
		{"name": "  Иван \t Гончаров\n", "note": " «Обломов»  "},
		{"name": "   ", "note": "пусто"},
		{"name": long, "note": ""},
		{"name": exact, "note": longNote},
		{"name": "иван гончаров", "note": "дубль"},
		{"name": "Урсула\u0007Ле Гуин", "note": "<b>Хьюго</b>"},
	}})
	cands, code := ParseOutput(answer(string(raw)), 20)
	if code != "" {
		t.Fatalf("code = %q", code)
	}
	if len(cands) != 3 {
		t.Fatalf("cands = %+v", cands)
	}
	if cands[0].Name != "Иван Гончаров" || cands[0].Note != "«Обломов»" {
		t.Errorf("trim/collapse: %+v", cands[0])
	}
	if cands[1].Name != exact || len([]rune(cands[1].Note)) != 200 {
		t.Errorf("150-rune name kept, note cut: %d runes", len([]rune(cands[1].Note)))
	}
	if cands[2].Name != "Урсула Ле Гуин" || cands[2].Note != "<b>Хьюго</b>" {
		t.Errorf("control char / markup: %+v", cands[2])
	}
}

func TestParseOutput_TruncatesToMaxNames(t *testing.T) {
	items := make([]string, 0, 14)
	for i := 0; i < 14; i++ {
		items = append(items, fmt.Sprintf(`{"name":"Автор %d","note":""}`, i))
	}
	cands, code := ParseOutput(answer(`{"authors":[`+strings.Join(items, ",")+`]}`), 10)
	if code != "" || len(cands) != 10 || cands[0].Name != "Автор 0" || cands[9].Name != "Автор 9" {
		t.Fatalf("code=%q cands=%+v", code, cands)
	}
}

func TestParseOutput_EmptyList(t *testing.T) {
	cands, code := ParseOutput(answer(`{"authors":[]}`), 20)
	if code != "" || cands == nil || len(cands) != 0 {
		t.Fatalf("code=%q cands=%v", code, cands)
	}
}

func TestPromptAndSchema(t *testing.T) {
	msg := UserMessage("русская классика", 12)
	if msg != "<request>\nрусская классика\n</request>\nМаксимум авторов: 12" {
		t.Errorf("user message = %q", msg)
	}
	if strings.Contains(SystemPrompt, "русская классика") || !strings.Contains(SystemPrompt, "<request>") {
		t.Error("system prompt must describe the request block and hold no request data")
	}
	schema := OutputSchema()
	if schema["additionalProperties"] != false {
		t.Error("schema must forbid extra fields")
	}
	item := schema["properties"].(map[string]any)["authors"].(map[string]any)["items"].(map[string]any)
	if item["additionalProperties"] != false || len(item["required"].([]string)) != 2 {
		t.Errorf("item schema = %v", item)
	}
}
