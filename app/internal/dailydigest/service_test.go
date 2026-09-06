package dailydigest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestParticipantChats(t *testing.T) {
	Convey("只选择开启摘要且参与 Bot 推送的群组并稳定排序", t, func() {
		chats := participantChats([]model.Chat{
			{ID: 3, Title: "群 C", SummaryEnabled: true, DeliveryMode: model.DeliveryModeBot},
			{ID: 2, Title: "群 B", SummaryEnabled: true, DeliveryMode: model.DeliveryModeDashboard},
			{ID: 1, Title: "群 A", SummaryEnabled: true, DeliveryMode: model.DeliveryModeBot},
			{ID: 4, Title: "群 D", SummaryEnabled: false, DeliveryMode: model.DeliveryModeBot},
		})

		So(chats, ShouldHaveLength, 2)
		So(chats[0].ID, ShouldEqual, int64(1))
		So(chats[1].ID, ShouldEqual, int64(3))
	})
}

func TestAllParticipantsDue(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	chats := []model.Chat{{SummaryTimeLocal: "08:00"}, {SummaryTimeLocal: "09:30"}}

	Convey("最后一个参与群组到达摘要时间后才允许生成", t, func() {
		So(allParticipantsDue(time.Date(2026, 9, 5, 9, 29, 0, 0, shanghai), chats, "Asia/Shanghai"), ShouldBeFalse)
		So(allParticipantsDue(time.Date(2026, 9, 5, 9, 30, 0, 0, shanghai), chats, "Asia/Shanghai"), ShouldBeTrue)
	})
}

func TestSummaryTerminal(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	settings := model.AppSettings{DefaultTimezone: "Asia/Shanghai", SummaryRetryLimit: 2}

	Convey("当天结束前生成的预览摘要尚未就绪", t, func() {
		item := model.Summary{
			Status:      model.SummaryStatusSucceeded,
			GeneratedAt: time.Date(2026, 9, 4, 23, 59, 0, 0, shanghai),
		}
		So(summaryTerminal(item, settings, "2026-09-04"), ShouldBeFalse)
	})

	Convey("当天结束后生成的成功摘要已经就绪", t, func() {
		item := model.Summary{
			Status:      model.SummaryStatusSucceeded,
			GeneratedAt: time.Date(2026, 9, 5, 0, 0, 0, 0, shanghai),
		}
		So(summaryTerminal(item, settings, "2026-09-04"), ShouldBeTrue)
	})

	Convey("仍有重试计划的失败摘要尚未进入终态", t, func() {
		next := time.Date(2026, 9, 5, 9, 5, 0, 0, shanghai)
		item := model.Summary{
			Status: model.SummaryStatusFailed, RetryCount: 1, NextRetryAt: &next,
		}
		So(summaryTerminal(item, settings, "2026-09-04"), ShouldBeFalse)
	})

	Convey("达到重试上限后允许降级生成", t, func() {
		next := time.Date(2026, 9, 5, 9, 5, 0, 0, shanghai)
		item := model.Summary{
			Status: model.SummaryStatusFailed, RetryCount: 2, NextRetryAt: &next,
		}
		So(summaryTerminal(item, settings, "2026-09-04"), ShouldBeTrue)
	})
}

func TestDailyDigestSource(t *testing.T) {
	selected := participant{chatID: 1, chatTitle: "群 A"}

	Convey("有正文的成功摘要会进入模型", t, func() {
		source := dailyDigestSource(selected, model.Summary{
			ID: 10, Status: model.SummaryStatusSucceeded, SourceMessageCount: 8, Content: "摘要",
		})
		So(source.Included, ShouldBeTrue)
		So(source.OmissionReason, ShouldBeEmpty)
	})

	Convey("无消息群组完成等待但不进入模型", t, func() {
		source := dailyDigestSource(selected, model.Summary{
			ID: 10, Status: model.SummaryStatusSucceeded, SourceMessageCount: 0,
		})
		So(source.Included, ShouldBeFalse)
		So(source.OmissionReason, ShouldEqual, "no_messages")
	})

	Convey("最终失败的群组会标记为遗漏", t, func() {
		source := dailyDigestSource(selected, model.Summary{ID: 10, Status: model.SummaryStatusFailed})
		So(source.Included, ShouldBeFalse)
		So(source.OmissionReason, ShouldEqual, "generation_failed")
	})
}

func TestGenerateDailyDigest(t *testing.T) {
	now := time.Date(2026, 9, 5, 9, 30, 0, 0, time.UTC)

	Convey("所有群组无消息时不调用模型并跳过投递", t, func() {
		item := model.DailyDigest{
			OmittedChatCount: 0,
			Sources:          []model.DailyDigestSource{{OmissionReason: "no_messages"}},
		}
		result := generate(t.Context(), digestTestSettings("http://127.0.0.1:1"), item, time.Second, now)

		So(result.Status, ShouldEqual, model.SummaryStatusSucceeded)
		So(result.ExecutionMode, ShouldEqual, "no_content")
		So(result.DeliverySkippedReason, ShouldEqual, "no_content")
	})

	Convey("只有一份有效摘要时直接复用且列出遗漏群组", t, func() {
		item := model.DailyDigest{
			OmittedChatCount: 1,
			Sources: []model.DailyDigestSource{
				{ChatTitle: "群 A", Included: true, Content: "单群正文", Model: "chat-model"},
				{ChatTitle: "群 B", OmissionReason: "generation_failed"},
			},
		}
		result := generate(t.Context(), digestTestSettings("http://127.0.0.1:1"), item, time.Second, now)

		So(result.Status, ShouldEqual, model.SummaryStatusSucceeded)
		So(result.ExecutionMode, ShouldEqual, "passthrough")
		So(result.Model, ShouldEqual, "chat-model")
		So(result.Content, ShouldContainSubstring, "单群正文")
		So(result.Content, ShouldContainSubstring, "群 B")
	})

	Convey("多份有效摘要会调用归并模型", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":"总览正文 [S001] [S002]"}}]}`))
		}))
		defer server.Close()

		item := model.DailyDigest{Sources: []model.DailyDigestSource{
			{ChatTitle: "群 A", Included: true, Content: strings.Repeat("摘要 A", 5)},
			{ChatTitle: "群 B", Included: true, Content: strings.Repeat("摘要 B", 5)},
		}}
		result := generate(t.Context(), digestTestSettings(server.URL), item, time.Second, now)

		So(result.Status, ShouldEqual, model.SummaryStatusSucceeded)
		So(result.ExecutionMode, ShouldEqual, "single")
		So(result.Content, ShouldEqual, "总览正文 [S001] [S002]")
	})

	Convey("临时模型错误会沿用摘要重试策略", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"temporarily unavailable"}}`))
		}))
		defer server.Close()

		settings := digestTestSettings(server.URL)
		settings.SummaryRetryLimit = 2
		settings.SummaryRetryBackoffBaseMinutes = 1
		settings.SummaryRetryBackoffMultiplier = 3
		item := model.DailyDigest{Sources: []model.DailyDigestSource{
			{ChatTitle: "群 A", Included: true, Content: "摘要 A"},
			{ChatTitle: "群 B", Included: true, Content: "摘要 B"},
		}}
		result := generate(t.Context(), settings, item, time.Second, now)

		So(result.Status, ShouldEqual, model.SummaryStatusFailed)
		So(result.NextRetryAt, ShouldNotBeNil)
		So(*result.NextRetryAt, ShouldEqual, now.Add(time.Minute))
	})
}

func TestBuildDailyDigestDeliveryMessage(t *testing.T) {
	item := model.DailyDigest{SummaryDate: "2026-09-04", Content: "正文"}

	Convey("Telegram 消息使用每日总览标题", t, func() {
		So(buildDeliveryMessage(model.LanguageZhCN, item), ShouldStartWith, "**每日总览 · 2026-09-04**")
		So(buildDeliveryMessage(model.LanguageEN, item), ShouldStartWith, "**Daily Digest · 2026-09-04**")
	})
	Convey("推送移除来源标记但保留正文、段落和网页原文", t, func() {
		item.Content = "重点 [S002] [S010]。\n\n- 结论[S001]\n- [注意] 保留"
		message := buildDeliveryMessage(model.LanguageZhCN, item)
		So(message, ShouldEqual, "**每日总览 · 2026-09-04**\n\n重点。\n\n- 结论\n- [注意] 保留")
		So(item.Content, ShouldContainSubstring, "[S002]")
	})
}

// TestDailyDigestRetryBackoff 验证总览沿用四次阶梯等待且耗尽后不再重试。
func TestDailyDigestRetryBackoff(t *testing.T) {
	now := time.Now()
	settings := model.AppSettings{SummaryRetryLimit: 4}
	for index, delay := range []time.Duration{time.Minute, 3 * time.Minute, 5 * time.Minute, 10 * time.Minute} {
		next := nextRetryAt(settings, index, now)
		if next == nil || !next.Equal(now.Add(delay)) {
			t.Fatalf("retry %d: %v", index+1, next)
		}
	}
	if nextRetryAt(settings, 4, now) != nil {
		t.Fatal("retry limit exceeded")
	}
}

func TestShouldAutomaticallyDeliver(t *testing.T) {
	Convey("只有尚未处理且未要求人工确认的成功总览会自动发送", t, func() {
		So(shouldAutomaticallyDeliver(model.DailyDigest{Status: model.SummaryStatusSucceeded}), ShouldBeTrue)
		So(shouldAutomaticallyDeliver(model.DailyDigest{
			Status: model.SummaryStatusSucceeded, DeliverySuppressed: true,
		}), ShouldBeFalse)
		So(shouldAutomaticallyDeliver(model.DailyDigest{
			Status: model.SummaryStatusSucceeded, DeliverySkippedReason: "no_content",
		}), ShouldBeFalse)
		So(shouldAutomaticallyDeliver(model.DailyDigest{Status: model.SummaryStatusFailed}), ShouldBeFalse)
	})
}

// digestTestSettings 提供使用本地假服务的最小模型配置。
func digestTestSettings(baseURL string) model.AppSettings {
	return model.AppSettings{
		OpenAIBaseURL: baseURL, OpenAIAPIKey: "test", OpenAIModel: "gpt-5.4",
		OpenAIRequestMode:         model.OpenAIRequestModeNonStream,
		OpenAIContextWindowMode:   model.ContextWindowModeManual,
		OpenAIContextWindowTokens: 64000, OpenAIOutputMode: model.OutputModeAuto,
		SummaryParallelism: 2, Language: model.LanguageZhCN,
	}
}
