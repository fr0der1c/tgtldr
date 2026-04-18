package scheduler

import (
	"testing"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDecideScheduledAction(t *testing.T) {
	deliveredAt := time.Date(2026, time.April, 17, 9, 0, 0, 0, time.UTC)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	readyAt := time.Date(2026, time.April, 18, 0, 1, 0, 0, shanghai)
	previewAt := time.Date(2026, time.April, 17, 16, 0, 0, 0, shanghai)

	tests := []struct {
		name     string
		chat     model.Chat
		summary  model.Summary
		found    bool
		expected scheduledAction
	}{
		{
			name:     "不存在摘要时重新生成",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeBot},
			found:    false,
			expected: scheduledActionGenerate,
		},
		{
			name:     "摘要未成功时重新生成",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeBot},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusFailed},
			expected: scheduledActionGenerate,
		},
		{
			name:     "Bot 模式且摘要完整时只发送",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeBot},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusSucceeded, SummaryDate: "2026-04-17", GeneratedAt: readyAt},
			expected: scheduledActionDeliver,
		},
		{
			name:     "Bot 模式但摘要在当天未结束前生成时重跑",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeBot},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusSucceeded, SummaryDate: "2026-04-17", GeneratedAt: previewAt},
			expected: scheduledActionGenerate,
		},
		{
			name:  "发送失败后继续重试发送",
			chat:  model.Chat{DeliveryMode: model.DeliveryModeBot},
			found: true,
			summary: model.Summary{
				Status:        model.SummaryStatusSucceeded,
				SummaryDate:   "2026-04-17",
				GeneratedAt:   readyAt,
				DeliveryError: "bot delivery is disabled",
			},
			expected: scheduledActionDeliver,
		},
		{
			name:  "Bot 模式且已发送时跳过",
			chat:  model.Chat{DeliveryMode: model.DeliveryModeBot},
			found: true,
			summary: model.Summary{
				Status:      model.SummaryStatusSucceeded,
				SummaryDate: "2026-04-17",
				GeneratedAt: readyAt,
				DeliveredAt: &deliveredAt,
			},
			expected: scheduledActionSkip,
		},
		{
			name:     "非 Bot 模式直接跳过发送",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeDashboard},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusSucceeded, SummaryDate: "2026-04-17", GeneratedAt: readyAt},
			expected: scheduledActionSkip,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual := decideScheduledAction(testCase.chat, testCase.summary, testCase.found, "Asia/Shanghai")
			if actual != testCase.expected {
				t.Fatalf("expected action %d, got %d", testCase.expected, actual)
			}
		})
	}
}

func TestSummaryReadyForDelivery(t *testing.T) {
	Convey("生成时间必须晚于摘要日期结束边界", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)

		So(summaryReadyForDelivery(model.Summary{
			SummaryDate: "2026-04-17",
			GeneratedAt: time.Date(2026, time.April, 17, 23, 59, 59, 0, shanghai),
		}, "Asia/Shanghai"), ShouldBeFalse)

		So(summaryReadyForDelivery(model.Summary{
			SummaryDate: "2026-04-17",
			GeneratedAt: time.Date(2026, time.April, 18, 0, 0, 0, 0, shanghai),
		}, "Asia/Shanghai"), ShouldBeTrue)
	})
}
