package rollup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	"github.com/fr0der1c/tgtldr/app/internal/model"
)

type contextTestRequest struct {
	Messages []struct{ Role, Content string }
}

// TestFinalContextPaths 验证直达、分块、超限回退和多层归并都只在最终请求附带历史。
func TestFinalContextPaths(t *testing.T) {
	for _, name := range []string{"direct", "chunked", "fallback", "hierarchical"} {
		t.Run(name, func(t *testing.T) {
			var mutex sync.Mutex
			var requests []contextTestRequest
			var failed, intermediate bool
			prompts := Prompts{Stage: "提取当天证据", Final: "编辑最终总览", FinalContext: "历史对照：2026-10-04\n旧话题"}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var payload contextTestRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || len(payload.Messages) != 2 {
					t.Errorf("invalid request: %+v, %v", payload, err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				mutex.Lock()
				defer mutex.Unlock()
				requests = append(requests, payload)
				if name == "fallback" && payload.Messages[0].Content == prompts.Final && !failed {
					failed = true
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":{"message":"context length exceeded","code":"context_length_exceeded"}}`))
					return
				}
				content := "最终总览 [S001]"
				if payload.Messages[0].Content == prompts.Stage {
					content = "当天证据 [S001]"
					if name == "hierarchical" && strings.Contains(payload.Messages[1].Content, "[S001]") {
						content = strings.Repeat("证", 1100)
					} else if name == "hierarchical" {
						intermediate = true
					}
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
				OpenAIContextWindowMode: model.ContextWindowModeManual, OpenAIContextWindowTokens: 8000,
				OpenAIOutputMode: model.OutputModeManual, OpenAIMaxOutputToken: 500, SummaryParallelism: 2,
			}
			content := "当天信息"
			if name == "chunked" || name == "hierarchical" {
				content = strings.Repeat(content, 7000)
			}
			if name == "hierarchical" {
				settings.OpenAIMaxOutputToken = 4000
			}
			result, err := Generate(t.Context(), settings, prompts, []Source{{Header: "[S001]\n", Content: content}}, time.Second)
			if err != nil || result.Content != "最终总览 [S001]" {
				t.Fatalf("unexpected result: %+v, %v", result, err)
			}
			mutex.Lock()
			defer mutex.Unlock()
			if name == "hierarchical" && !intermediate {
				t.Fatal("test did not exercise an intermediate merge")
			}
			if name == "fallback" && (!failed || result.ExecutionMode != "fallback_chunked") {
				t.Fatal("test did not exercise context-limit fallback")
			}
			for index, request := range requests {
				system, input := request.Messages[0].Content, request.Messages[1].Content
				if system == prompts.Stage && strings.Contains(input, "旧话题") {
					t.Fatal("history entered evidence extraction or intermediate merging")
				}
				if system == prompts.Final && !strings.Contains(input, prompts.FinalContext) {
					t.Fatal("final request lost history")
				}
				reserve := settings.OpenAIMaxOutputToken
				if system == prompts.Stage {
					reserve = min(reserve, stageOutputReserve)
				}
				if !llmcontext.PlanRequest(llmcontext.NewCounter("custom-model"), 8000, system, input, reserve).Fits {
					t.Fatalf("request %d exceeded budget", index)
				}
			}
		})
	}
}
