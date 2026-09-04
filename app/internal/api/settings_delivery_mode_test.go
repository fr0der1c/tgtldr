package api

import (
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestApplyBotSummaryDeliveryModeTransition(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	now := time.Date(2026, time.September, 4, 10, 30, 0, 0, shanghai)

	Convey("包装方式变化时从当天消息开始使用新配置", t, func() {
		current := model.AppSettings{
			BotSummaryDeliveryMode:              model.BotSummaryDeliveryModePerChat,
			PreviousBotSummaryDeliveryMode:      model.BotSummaryDeliveryModePerChat,
			BotSummaryDeliveryModeEffectiveDate: "1970-01-01",
		}
		next := applyBotSummaryDeliveryModeTransition(model.AppSettings{
			DefaultTimezone:        "Asia/Shanghai",
			BotSummaryDeliveryMode: model.BotSummaryDeliveryModeDailyDigest,
		}, current, now)

		So(next.PreviousBotSummaryDeliveryMode, ShouldEqual, model.BotSummaryDeliveryModePerChat)
		So(next.BotSummaryDeliveryModeEffectiveDate, ShouldEqual, "2026-09-04")
		So(model.ResolveBotSummaryDeliveryMode(next, "2026-09-03"), ShouldEqual, model.BotSummaryDeliveryModePerChat)
		So(model.ResolveBotSummaryDeliveryMode(next, "2026-09-04"), ShouldEqual, model.BotSummaryDeliveryModeDailyDigest)
	})

	Convey("同一天反复切换时仍保留前一天实际生效的配置", t, func() {
		current := model.AppSettings{
			BotSummaryDeliveryMode:              model.BotSummaryDeliveryModeDailyDigest,
			PreviousBotSummaryDeliveryMode:      model.BotSummaryDeliveryModePerChat,
			BotSummaryDeliveryModeEffectiveDate: "2026-09-04",
		}
		next := applyBotSummaryDeliveryModeTransition(model.AppSettings{
			DefaultTimezone:        "Asia/Shanghai",
			BotSummaryDeliveryMode: model.BotSummaryDeliveryModePerChat,
		}, current, now)

		So(next.PreviousBotSummaryDeliveryMode, ShouldEqual, model.BotSummaryDeliveryModePerChat)
		So(model.ResolveBotSummaryDeliveryMode(next, "2026-09-03"), ShouldEqual, model.BotSummaryDeliveryModePerChat)
		So(model.ResolveBotSummaryDeliveryMode(next, "2026-09-04"), ShouldEqual, model.BotSummaryDeliveryModePerChat)
	})

	Convey("包装方式不变时保留原生效日期", t, func() {
		current := model.AppSettings{
			BotSummaryDeliveryMode:              model.BotSummaryDeliveryModeDailyDigest,
			PreviousBotSummaryDeliveryMode:      model.BotSummaryDeliveryModePerChat,
			BotSummaryDeliveryModeEffectiveDate: "2026-08-10",
		}
		next := applyBotSummaryDeliveryModeTransition(model.AppSettings{
			DefaultTimezone:        "Asia/Shanghai",
			BotSummaryDeliveryMode: model.BotSummaryDeliveryModeDailyDigest,
		}, current, now)

		So(next.PreviousBotSummaryDeliveryMode, ShouldEqual, model.BotSummaryDeliveryModePerChat)
		So(next.BotSummaryDeliveryModeEffectiveDate, ShouldEqual, "2026-08-10")
	})
}
