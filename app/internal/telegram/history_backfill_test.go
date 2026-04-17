package telegram

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParseHistoryRange(t *testing.T) {
	Convey("历史回补日期范围会按本地时区解析并包含结束日", t, func() {
		start, end, err := parseHistoryRange("2026-04-01", "2026-04-03", "Asia/Shanghai")

		So(err, ShouldBeNil)
		So(start, ShouldEqual, time.Date(2026, 3, 31, 16, 0, 0, 0, time.UTC))
		So(end, ShouldEqual, time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC))
	})

	Convey("结束日期早于开始日期时会报错", t, func() {
		_, _, err := parseHistoryRange("2026-04-03", "2026-04-01", "Asia/Shanghai")

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "结束日期不能早于开始日期")
	})
}
