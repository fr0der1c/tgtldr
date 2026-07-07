package scheduler

import (
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
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
			name:     "失败摘要没有重试计划时跳过",
			chat:     model.Chat{DeliveryMode: model.DeliveryModeBot},
			found:    true,
			summary:  model.Summary{Status: model.SummaryStatusFailed},
			expected: scheduledActionSkip,
		},
		{
			name:  "失败摘要到达重试时间时自动重试",
			chat:  model.Chat{DeliveryMode: model.DeliveryModeBot},
			found: true,
			summary: model.Summary{
				Status:     model.SummaryStatusFailed,
				RetryCount: 1,
				NextRetryAt: timePtr(time.Date(
					2026,
					time.April,
					18,
					8,
					59,
					0,
					0,
					shanghai,
				)),
			},
			expected: scheduledActionRetry,
		},
		{
			name:  "失败摘要未到重试时间时跳过",
			chat:  model.Chat{DeliveryMode: model.DeliveryModeBot},
			found: true,
			summary: model.Summary{
				Status:     model.SummaryStatusFailed,
				RetryCount: 1,
				NextRetryAt: timePtr(time.Date(
					2026,
					time.April,
					18,
					9,
					1,
					0,
					0,
					shanghai,
				)),
			},
			expected: scheduledActionSkip,
		},
		{
			name:  "失败摘要达到重试上限后跳过",
			chat:  model.Chat{DeliveryMode: model.DeliveryModeBot},
			found: true,
			summary: model.Summary{
				Status:     model.SummaryStatusFailed,
				RetryCount: 2,
				NextRetryAt: timePtr(time.Date(
					2026,
					time.April,
					18,
					8,
					59,
					0,
					0,
					shanghai,
				)),
			},
			expected: scheduledActionSkip,
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
			actual := decideScheduledAction(
				testCase.chat,
				testCase.summary,
				testCase.found,
				"Asia/Shanghai",
				model.AppSettings{SummaryRetryLimit: 2},
				time.Date(2026, time.April, 18, 9, 0, 0, 0, shanghai),
			)
			if actual != testCase.expected {
				t.Fatalf("expected action %d, got %d", testCase.expected, actual)
			}
		})
	}
}

func TestSummaryRetryBackoff(t *testing.T) {
	Convey("重试退避按起始间隔和倍率计算", t, func() {
		settings := model.AppSettings{
			SummaryRetryLimit:              3,
			SummaryRetryBackoffBaseMinutes: 1,
			SummaryRetryBackoffMultiplier:  3,
		}

		So(summaryRetryBackoffDelay(settings, 1), ShouldEqual, time.Minute)
		So(summaryRetryBackoffDelay(settings, 2), ShouldEqual, 3*time.Minute)
		So(summaryRetryBackoffDelay(settings, 3), ShouldEqual, 9*time.Minute)
	})

	Convey("倍率为 1 时固定间隔重试", t, func() {
		settings := model.AppSettings{
			SummaryRetryLimit:              3,
			SummaryRetryBackoffBaseMinutes: 2,
			SummaryRetryBackoffMultiplier:  1,
		}

		So(summaryRetryBackoffDelay(settings, 1), ShouldEqual, 2*time.Minute)
		So(summaryRetryBackoffDelay(settings, 3), ShouldEqual, 2*time.Minute)
	})

	Convey("极大重试次数不会导致 duration 溢出", t, func() {
		settings := model.AppSettings{
			SummaryRetryLimit:              1000,
			SummaryRetryBackoffBaseMinutes: 1,
			SummaryRetryBackoffMultiplier:  3,
		}

		So(summaryRetryBackoffDelay(settings, 1000), ShouldEqual, maxRetryBackoffDuration)
	})
}

func timePtr(value time.Time) *time.Time {
	return &value
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

func TestDatesInRange(t *testing.T) {
	Convey("日期范围会包含首尾两天", t, func() {
		So(datesInRange("2026-04-10", "2026-04-12", "Asia/Shanghai"), ShouldResemble, []string{
			"2026-04-10",
			"2026-04-11",
			"2026-04-12",
		})
	})
}

func TestIsRepairableEmptySummary(t *testing.T) {
	Convey("只有成功且消息数和分块数都为零的摘要才会被修复", t, func() {
		So(isRepairableEmptySummary(model.Summary{
			Status:             model.SummaryStatusSucceeded,
			SourceMessageCount: 0,
			ChunkCount:         0,
		}), ShouldBeTrue)

		So(isRepairableEmptySummary(model.Summary{
			Status:             model.SummaryStatusFailed,
			SourceMessageCount: 0,
			ChunkCount:         0,
		}), ShouldBeFalse)

		So(isRepairableEmptySummary(model.Summary{
			Status:             model.SummaryStatusSucceeded,
			SourceMessageCount: 1,
			ChunkCount:         0,
		}), ShouldBeFalse)
	})
}

func TestRollingSummaryWindow(t *testing.T) {
	Convey("rolling summary uses today's local midnight through current time", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)

		now := time.Date(2026, time.July, 7, 15, 30, 0, 0, shanghai)
		window, ok := rollingSummaryWindow(now, "Asia/Shanghai")

		So(ok, ShouldBeTrue)
		So(window.SummaryDate, ShouldEqual, "2026-07-07")
		So(window.Start, ShouldResemble, time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC())
		So(window.End, ShouldResemble, now.UTC())
	})
}

func TestRollingSummaryEligibility(t *testing.T) {
	Convey("rolling summary skips until interval elapses", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)
		now := time.Date(2026, time.July, 7, 15, 30, 0, 0, shanghai)
		windowStart := time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC()
		lastEnd := time.Date(2026, time.July, 7, 13, 0, 0, 0, shanghai)

		eligible := rollingSummaryEligible(model.Chat{
			RollingSummaryEnabled:         true,
			RollingSummaryIntervalMinutes: 180,
			RollingSummaryMaxPerDay:       5,
		}, model.Summary{
			SummaryType: model.SummaryTypeRolling,
			WindowEnd:   &lastEnd,
		}, true, 1, 10, windowStart, now)

		So(eligible, ShouldBeFalse)
	})

	Convey("rolling summary runs when interval elapsed and new messages exist", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)
		now := time.Date(2026, time.July, 7, 16, 30, 0, 0, shanghai)
		windowStart := time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC()
		lastEnd := time.Date(2026, time.July, 7, 13, 0, 0, 0, shanghai)

		eligible := rollingSummaryEligible(model.Chat{
			RollingSummaryEnabled:         true,
			RollingSummaryIntervalMinutes: 180,
			RollingSummaryMaxPerDay:       5,
		}, model.Summary{
			SummaryType: model.SummaryTypeRolling,
			WindowEnd:   &lastEnd,
		}, true, 1, 10, windowStart, now)

		So(eligible, ShouldBeTrue)
	})

	Convey("rolling summary respects max sends per day", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)
		now := time.Date(2026, time.July, 7, 16, 30, 0, 0, shanghai)
		windowStart := time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC()

		eligible := rollingSummaryEligible(model.Chat{
			RollingSummaryEnabled:         true,
			RollingSummaryIntervalMinutes: 180,
			RollingSummaryMaxPerDay:       5,
		}, model.Summary{}, false, 5, 10, windowStart, now)

		So(eligible, ShouldBeFalse)
	})

	Convey("rolling summary requires new messages", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)
		now := time.Date(2026, time.July, 7, 16, 30, 0, 0, shanghai)
		windowStart := time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC()

		eligible := rollingSummaryEligible(model.Chat{
			RollingSummaryEnabled:         true,
			RollingSummaryIntervalMinutes: 180,
			RollingSummaryMaxPerDay:       5,
		}, model.Summary{}, false, 0, 0, windowStart, now)

		So(eligible, ShouldBeFalse)
	})

	Convey("first rolling summary waits for the configured interval after midnight", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)
		windowStart := time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC()
		now := time.Date(2026, time.July, 7, 1, 30, 0, 0, shanghai)

		eligible := rollingSummaryEligible(model.Chat{
			RollingSummaryEnabled:         true,
			RollingSummaryIntervalMinutes: 180,
			RollingSummaryMaxPerDay:       5,
		}, model.Summary{}, false, 0, 10, windowStart, now)

		So(eligible, ShouldBeFalse)
	})

	Convey("first rolling summary runs after the configured interval", t, func() {
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)
		windowStart := time.Date(2026, time.July, 7, 0, 0, 0, 0, shanghai).UTC()
		now := time.Date(2026, time.July, 7, 3, 0, 0, 0, shanghai)

		eligible := rollingSummaryEligible(model.Chat{
			RollingSummaryEnabled:         true,
			RollingSummaryIntervalMinutes: 180,
			RollingSummaryMaxPerDay:       5,
		}, model.Summary{}, false, 0, 10, windowStart, now)

		So(eligible, ShouldBeTrue)
	})
}
