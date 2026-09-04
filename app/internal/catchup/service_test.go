package catchup

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

func TestValidateDateRange(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	Convey("最多允许九十个已经结束的自然日", t, func() {
		So(ValidateDateRange("2026-05-25", "2026-08-22", "Asia/Shanghai", now), ShouldBeNil)
		So(ValidateDateRange("2026-05-24", "2026-08-22", "Asia/Shanghai", now), ShouldEqual, ErrDateRangeTooLong)
	})

	Convey("截止日期不能包含今天", t, func() {
		So(ValidateDateRange("2026-08-20", "2026-08-23", "Asia/Shanghai", now), ShouldEqual, ErrDateRangeInFuture)
	})

	Convey("开始日期不能晚于截止日期", t, func() {
		So(ValidateDateRange("2026-08-22", "2026-08-20", "Asia/Shanghai", now), ShouldEqual, ErrInvalidDateRange)
	})
}

func TestBuildRollupSources(t *testing.T) {
	Convey("来源编号应按输入顺序稳定写入", t, func() {
		units := buildRollupSources([]model.CatchUpSource{
			{ChatTitle: "群 A", SummaryDate: "2026-08-20", Content: "摘要 A"},
			{ChatTitle: "群 B", SummaryDate: "2026-08-21", Content: "摘要 B"},
		}, model.LanguageZhCN)

		So(units, ShouldHaveLength, 2)
		So(units[0].Header, ShouldContainSubstring, "[S001]")
		So(units[1].Header, ShouldContainSubstring, "[S002]")
	})
}

func TestBuildDeliveryMessage(t *testing.T) {
	item := model.CatchUp{FromDate: "2026-08-01", ToDate: "2026-08-07", Content: "正文"}

	Convey("中文投递使用快速回顾标题", t, func() {
		So(buildDeliveryMessage(model.LanguageZhCN, item), ShouldStartWith, "**快速回顾 · 2026-08-01 – 2026-08-07**")
	})

	Convey("英文投递保留 Catch Up 标题", t, func() {
		So(buildDeliveryMessage(model.LanguageEN, item), ShouldStartWith, "**Catch Up · 2026-08-01 – 2026-08-07**")
	})
}

func TestGenerateCatchUp(t *testing.T) {
	Convey("输入能够放入上下文时只调用一次模型", t, func() {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":"完整 Catch Up"}}]}`))
		}))
		defer server.Close()

		result, err := generate(t.Context(), catchUpTestSettings(server.URL), catchUpTestSources(2, 4), time.Second)

		So(err, ShouldBeNil)
		So(requestCount, ShouldEqual, 1)
		So(result.executionMode, ShouldEqual, "single")
		So(result.chunkCount, ShouldEqual, 1)
		So(result.content, ShouldEqual, "完整 Catch Up")
	})

	Convey("单次请求真实超限时会降级为多批生成", t, func() {
		var mu sync.Mutex
		requestCount := 0
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
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if current == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`))
				return
			}
			content := "阶段笔记"
			if len(payload.Messages) > 0 && strings.Contains(payload.Messages[0].Content, "Catch Up 编辑器") {
				content = "兜底 Catch Up"
			}
			_, _ = fmt.Fprintf(w, `{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":%q}}]}`, content)
		}))
		defer server.Close()

		result, err := generate(t.Context(), catchUpTestSettings(server.URL), catchUpTestSources(18, 100), 3*time.Second)

		So(err, ShouldBeNil)
		So(result.executionMode, ShouldEqual, "fallback_chunked")
		So(result.fallbackReason, ShouldEqual, "context_limit_error")
		So(result.chunkCount, ShouldBeGreaterThan, 1)
		So(result.content, ShouldEqual, "兜底 Catch Up")
		So(requestCount, ShouldBeGreaterThanOrEqualTo, 4)
	})
}

func catchUpTestSettings(baseURL string) model.AppSettings {
	return model.AppSettings{
		OpenAIBaseURL: baseURL, OpenAIAPIKey: "test", OpenAIModel: "gpt-5.4",
		OpenAIRequestMode:         model.OpenAIRequestModeNonStream,
		OpenAIContextWindowMode:   model.ContextWindowModeManual,
		OpenAIContextWindowTokens: 64000,
		OpenAIOutputMode:          model.OutputModeAuto,
		SummaryParallelism:        3, Language: model.LanguageZhCN,
	}
}

func catchUpTestSources(count int, repetitions int) []model.CatchUpSource {
	items := make([]model.CatchUpSource, 0, count)
	for index := 0; index < count; index++ {
		items = append(items, model.CatchUpSource{
			SummaryID: int64(index + 1), ChatID: int64(index%3 + 1),
			ChatTitle: fmt.Sprintf("群 %d", index%3+1), SummaryDate: fmt.Sprintf("2026-08-%02d", index%20+1),
			Content: strings.Repeat("讨论了产品体验、价格变化和仍待确认的结论。", repetitions),
		})
	}
	return items
}
