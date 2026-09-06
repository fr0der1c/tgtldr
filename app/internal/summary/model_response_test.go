package summary

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// TestDailyModelResponse 验证空正文不会成功落库，且成功时分别保留请求与返回模型。
func TestDailyModelResponse(t *testing.T) {
	for _, content := range []string{"", "有效摘要"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"model":"terra","choices":[{"message":{"content":"` + content + `"}}]}`))
		}))
		messages, lookup := dailyTestMessages(2, 3)
		input := dailyTestResult()
		input.RequestedModel = input.Model
		service := &Service{openAITimeout: time.Second}
		result, err := service.generateDailySummary(t.Context(), dailyTestSettings(server.URL), model.Chat{}, time.UTC, messages, lookup, input)
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if result.RequestedModel != input.Model {
			t.Fatal("requested model was overwritten")
		}
		if content == "" {
			if result.Status != model.SummaryStatusFailed || !result.RetryableError || result.ErrorContext == "" {
				t.Fatalf("empty response must be retryable with context: %+v", result)
			}
		} else if result.Status != model.SummaryStatusSucceeded || result.ReturnedModel != "terra" || result.Model != "terra" {
			t.Fatalf("model attribution: %+v", result)
		}
	}
}
