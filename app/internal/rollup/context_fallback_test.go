package rollup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// TestHistoryShrinksAfterContextLimit 验证最终编辑再次超限时继续缩减历史并保留当天证据。
func TestHistoryShrinksAfterContextLimit(t *testing.T) {
	counter := llmcontext.NewCounter("custom-model")
	finalInputs := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var payload contextTestRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || len(payload.Messages) != 2 {
			t.Errorf("invalid request: %+v, %v", payload, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		input := payload.Messages[1].Content
		if counter.Count(input) > 200 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"context_length_exceeded","message":"context length exceeded"}}`))
			return
		}
		content := "当天证据 [S001]"
		if payload.Messages[0].Content == "最终编辑" {
			finalInputs <- input
			content = "最终总览"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}},
		})
	}))
	defer server.Close()
	settings := model.AppSettings{
		OpenAIBaseURL: server.URL, OpenAIAPIKey: "test", OpenAIModel: "custom-model",
		OpenAIRequestMode:       model.OpenAIRequestModeNonStream,
		OpenAIContextWindowMode: model.ContextWindowModeManual, OpenAIContextWindowTokens: 12000,
		OpenAIOutputMode: model.OutputModeManual, OpenAIMaxOutputToken: 500,
	}
	prompts := Prompts{
		Stage: "提取证据", Final: "最终编辑",
		FinalContext: "总览日期：2026-10-05\n历史对照：2026-10-04\n" + strings.Repeat("旧信息", 1800),
	}
	result, err := Generate(t.Context(), settings, prompts, []Source{{Header: "[S001]\n", Content: "当天证据"}}, time.Second)
	if err != nil || result.Content != "最终总览" || result.ExecutionMode != "fallback_chunked" {
		t.Fatalf("unexpected context-limit result: %+v, %v", result, err)
	}
	select {
	case input := <-finalInputs:
		if !strings.Contains(input, "当天证据") || !strings.Contains(input, "2026-10-05") || !strings.Contains(input, "Comparison context truncated") {
			t.Fatalf("missing evidence or comparison boundary: %s", input)
		}
	default:
		t.Fatal("no successful final request")
	}
}
