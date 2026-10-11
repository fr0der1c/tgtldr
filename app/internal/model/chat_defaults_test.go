package model

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestChatDefaultsNormalize(t *testing.T) {
	Convey("默认配置会去除空白并校验交付方式和摘要时间", t, func() {
		normalized, err := ChatDefaults{DeliveryMode: DeliveryModeBot, SummaryTimeLocal: " 08:30 ", ModelOverride: " gpt-4.1-mini "}.Normalize()
		So(err, ShouldBeNil)
		So(normalized.SummaryTimeLocal, ShouldEqual, "08:30")
		So(normalized.ModelOverride, ShouldEqual, "gpt-4.1-mini")

		normalized, err = ChatDefaults{DeliveryMode: DeliveryModeDashboard, SummaryTimeLocal: "9:05"}.Normalize()
		So(err, ShouldBeNil)
		So(normalized.SummaryTimeLocal, ShouldEqual, "09:05")

		_, err = ChatDefaults{DeliveryMode: "email", SummaryTimeLocal: "08:30"}.Normalize()
		So(err, ShouldNotBeNil)

		_, err = ChatDefaults{DeliveryMode: DeliveryModeDashboard, SummaryTimeLocal: "25:00"}.Normalize()
		So(err, ShouldNotBeNil)
	})

	Convey("内置默认值与原先新群组的初始配置一致且有效", t, func() {
		defaults, err := DefaultChatDefaults().Normalize()
		So(err, ShouldBeNil)
		So(defaults.DeliveryMode, ShouldEqual, DeliveryModeDashboard)
		So(defaults.SummaryTimeLocal, ShouldEqual, "09:00")
		So(defaults.KeepBotMessages, ShouldBeTrue)
	})
}
