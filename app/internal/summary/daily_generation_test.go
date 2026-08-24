package summary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGenerateDailySummaryAdaptiveContext(t *testing.T) {
	Convey("完整群聊能够放入上下文时只生成一次", t, func() {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":"最终日报"}}]}`))
		}))
		defer server.Close()

		messages, lookup := dailyTestMessages(2, 3)
		service := &Service{openAITimeout: time.Second}
		result, err := service.generateDailySummary(
			t.Context(), dailyTestSettings(server.URL), model.Chat{}, time.UTC,
			messages, lookup, dailyTestResult(),
		)

		So(err, ShouldBeNil)
		So(result.Status, ShouldEqual, model.SummaryStatusSucceeded)
		So(result.Content, ShouldEqual, "最终日报")
		So(result.ChunkCount, ShouldEqual, 1)
		So(requestCount, ShouldEqual, 1)
	})

	Convey("单次请求真实超限时会缩小分块并完成最终合并", t, func() {
		var mu sync.Mutex
		requestCount := 0
		firstWasDirect := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.NewDecoder(req.Body).Decode(&payload)
			mu.Lock()
			requestCount++
			current := requestCount
			if current == 1 && len(payload.Messages) > 0 {
				firstWasDirect = strings.Contains(payload.Messages[0].Content, "最终摘要器")
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if current == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`))
				return
			}
			content := "阶段摘要"
			if len(payload.Messages) > 0 && strings.Contains(payload.Messages[0].Content, "最终摘要器") {
				content = "兜底日报"
			}
			_, _ = fmt.Fprintf(w, `{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":%q}}]}`, content)
		}))
		defer server.Close()

		messages, lookup := dailyTestMessages(20, 100)
		service := &Service{openAITimeout: 3 * time.Second}
		result, err := service.generateDailySummary(
			t.Context(), dailyTestSettings(server.URL), model.Chat{}, time.UTC,
			messages, lookup, dailyTestResult(),
		)

		So(err, ShouldBeNil)
		So(result.Status, ShouldEqual, model.SummaryStatusSucceeded)
		So(result.Content, ShouldEqual, "兜底日报")
		So(result.ChunkCount, ShouldBeGreaterThan, 1)
		So(firstWasDirect, ShouldBeTrue)
		So(requestCount, ShouldBeGreaterThanOrEqualTo, 4)
	})
}

func dailyTestSettings(baseURL string) model.AppSettings {
	return model.AppSettings{
		OpenAIBaseURL: baseURL, OpenAIAPIKey: "test", OpenAIModel: "gpt-5.4",
		OpenAIRequestMode:         model.OpenAIRequestModeNonStream,
		OpenAIContextWindowMode:   model.ContextWindowModeManual,
		OpenAIContextWindowTokens: 64000,
		OpenAIOutputMode:          model.OutputModeAuto,
		SummaryParallelism:        3, Language: model.LanguageZhCN,
	}
}

func dailyTestMessages(count int, repetitions int) ([]model.Message, map[int]model.Message) {
	base := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	messages := make([]model.Message, 0, count)
	lookup := make(map[int]model.Message, count)
	for index := 0; index < count; index++ {
		message := model.Message{
			TelegramMessageID: index + 1,
			SenderName:        fmt.Sprintf("用户 %d", index%4+1),
			TextContent:       strings.Repeat("讨论了产品体验、价格变化和仍待确认的结论。", repetitions),
			MessageTime:       base.Add(time.Duration(index) * time.Minute),
		}
		messages = append(messages, message)
		lookup[message.TelegramMessageID] = message
	}
	return messages, lookup
}

func dailyTestResult() model.Summary {
	return model.Summary{
		ChatID: 1, SummaryDate: "2026-08-20", Status: model.SummaryStatusSucceeded,
		Model: "gpt-5.4", GeneratedAt: time.Now(),
	}
}
