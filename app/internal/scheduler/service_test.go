package scheduler

import (
	"testing"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
)

func TestDecideScheduledAction(t *testing.T) {
	deliveredAt := time.Date(2026, time.April, 17, 9, 0, 0, 0, time.UTC)

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
			name:     "Bot 模式且未发送时只发送",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeBot},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusSucceeded},
			expected: scheduledActionDeliver,
		},
		{
			name:  "发送失败后继续重试发送",
			chat:  model.Chat{DeliveryMode: model.DeliveryModeBot},
			found: true,
			summary: model.Summary{
				Status:        model.SummaryStatusSucceeded,
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
				DeliveredAt: &deliveredAt,
			},
			expected: scheduledActionSkip,
		},
		{
			name:     "非 Bot 模式直接跳过发送",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeDashboard},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusSucceeded},
			expected: scheduledActionSkip,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual := decideScheduledAction(testCase.chat, testCase.summary, testCase.found)
			if actual != testCase.expected {
				t.Fatalf("expected action %d, got %d", testCase.expected, actual)
			}
		})
	}
}
