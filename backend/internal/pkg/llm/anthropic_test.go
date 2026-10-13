package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
)

// newTestServer answers every request with status and body and records the
// last request body. It never touches the network beyond loopback.
func newTestServer(t *testing.T, status int, body string, got *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if got != nil {
			*got = map[string]any{}
			if err := json.Unmarshal(raw, got); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(srv *httptest.Server) *Anthropic {
	return NewAnthropic("test-key", "claude-haiku-5-5", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
}

func messageBody(stopReason, text, stopDetails string) string {
	m := map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-haiku-5-5",
		"stop_reason": stopReason, "stop_sequence": nil,
		"content": []any{},
		"usage":   map[string]any{"input_tokens": 1200, "output_tokens": 80, "cache_read_input_tokens": 1000},
	}
	if text != "" {
		m["content"] = []any{
			map[string]any{"type": "thinking", "thinking": "hmm", "signature": "sig"},
			map[string]any{"type": "text", "text": text},
		}
	}
	if stopDetails != "" {
		m["stop_details"] = map[string]any{"type": "refusal", "category": "cyber", "explanation": stopDetails}
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}

var testSchema = map[string]any{"type": "object", "additionalProperties": false}

func TestAnthropic_RequestWireFormat(t *testing.T) {
	var got map[string]any
	srv := newTestServer(t, 200, messageBody("end_turn", `{"verdict":"publish"}`, ""), &got)

	_, err := newTestClient(srv).Structured(context.Background(), StructuredRequest{System: "rules", User: "<book>{}</book>", Schema: testSchema})
	if err != nil {
		t.Fatalf("Structured: %v", err)
	}
	if got["model"] != "claude-haiku-5-5" {
		t.Errorf("model = %v", got["model"])
	}
	if got["max_tokens"] != float64(DefaultMaxTokens) {
		t.Errorf("max_tokens = %v", got["max_tokens"])
	}
	if _, ok := got["output_format"]; ok {
		t.Error("deprecated output_format must not be sent")
	}
	if _, ok := got["tool_choice"]; ok {
		t.Error("tool_choice must not be sent")
	}
	oc, _ := got["output_config"].(map[string]any)
	if oc["effort"] != "low" {
		t.Errorf("output_config.effort = %v", oc["effort"])
	}
	format, _ := oc["format"].(map[string]any)
	if format["type"] != "json_schema" || format["schema"] == nil {
		t.Errorf("output_config.format = %v", format)
	}
	system, _ := got["system"].([]any)
	if len(system) != 1 {
		t.Fatalf("system = %v", got["system"])
	}
	sys0, _ := system[0].(map[string]any)
	cc, _ := sys0["cache_control"].(map[string]any)
	if sys0["text"] != "rules" || cc["type"] != "ephemeral" {
		t.Errorf("system block = %v", sys0)
	}
	msgs, _ := got["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected exactly one message (no prefill), got %v", msgs)
	}
	m0, _ := msgs[0].(map[string]any)
	if m0["role"] != "user" {
		t.Errorf("message role = %v", m0["role"])
	}
	content, _ := m0["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("user content = %v", m0["content"])
	}
	c0, _ := content[0].(map[string]any)
	if c0["type"] != "text" || c0["text"] != "<book>{}</book>" {
		t.Errorf("user content block = %v", c0)
	}
}

func TestAnthropic_ResponseMapping(t *testing.T) {
	tests := []struct {
		name, stop, text, details string
		wantJSON, wantDetails     string
	}{
		{name: "end_turn", stop: "end_turn", text: `{"verdict":"publish"}`, wantJSON: `{"verdict":"publish"}`},
		{name: "max_tokens", stop: "max_tokens", text: `{"verd`, wantJSON: `{"verd`},
		{name: "refusal", stop: "refusal", details: "policy", wantDetails: "cyber: policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, 200, messageBody(tt.stop, tt.text, tt.details), nil)
			resp, err := newTestClient(srv).Structured(context.Background(), StructuredRequest{System: "s", User: "u", Schema: testSchema})
			if err != nil {
				t.Fatalf("Structured: %v", err)
			}
			if resp.StopReason != tt.stop || string(resp.JSON) != tt.wantJSON || resp.StopDetails != tt.wantDetails {
				t.Errorf("got stop=%q json=%q details=%q", resp.StopReason, resp.JSON, resp.StopDetails)
			}
			if resp.Model != "claude-haiku-5-5" || resp.InputTokens != 1200 || resp.OutputTokens != 80 || resp.CacheReadTokens != 1000 {
				t.Errorf("meta = %+v", resp)
			}
		})
	}
}

func TestAnthropic_ErrorMapping(t *testing.T) {
	errBody := `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`
	for _, status := range []int{429, 500, 503, 529} {
		srv := newTestServer(t, status, errBody, nil)
		_, err := newTestClient(srv).Structured(context.Background(), StructuredRequest{System: "s", User: "u", Schema: testSchema})
		if !errors.Is(err, ErrTransient) {
			t.Errorf("status %d: expected ErrTransient, got %v", status, err)
		}
	}
	srv := newTestServer(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`, nil)
	_, err := newTestClient(srv).Structured(context.Background(), StructuredRequest{System: "s", User: "u", Schema: testSchema})
	if err == nil || errors.Is(err, ErrTransient) {
		t.Errorf("400 must be a non-transient error, got %v", err)
	}

	srv = newTestServer(t, 200, "{}", nil)
	srv.Close()
	_, err = newTestClient(srv).Structured(context.Background(), StructuredRequest{System: "s", User: "u", Schema: testSchema})
	if !errors.Is(err, ErrTransient) {
		t.Errorf("network error: expected ErrTransient, got %v", err)
	}
}
