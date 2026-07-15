package api

import (
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestChatMessageCursor(t *testing.T) {
	Convey("消息游标应该保留时间和 Telegram 消息 ID", t, func() {
		messageTime := time.Date(2026, 7, 14, 12, 34, 56, 123, time.UTC)
		encoded := encodeMessageCursor(model.Message{
			MessageTime: messageTime, TelegramMessageID: 42,
		})

		decoded, err := decodeMessageCursor(encoded)

		So(err, ShouldBeNil)
		So(decoded.MessageTime, ShouldResemble, messageTime)
		So(decoded.TelegramMessageID, ShouldEqual, 42)
	})

	Convey("非法消息游标应该被拒绝", t, func() {
		_, err := decodeMessageCursor("not-a-cursor")

		So(err, ShouldNotBeNil)
	})
}

func TestParseChatMessageLimit(t *testing.T) {
	Convey("未指定 limit 时默认加载最近 2000 条", t, func() {
		limit, err := parseChatMessageLimit("")

		So(err, ShouldBeNil)
		So(limit, ShouldEqual, 2000)
	})

	Convey("limit 不允许超过 2000", t, func() {
		_, err := parseChatMessageLimit("2001")

		So(err, ShouldNotBeNil)
	})
}

func TestParseOptionalBool(t *testing.T) {
	Convey("筛选开关应该接受 URL 中的布尔值", t, func() {
		enabled, err := parseOptionalBool("1", "filters")
		So(err, ShouldBeNil)
		So(enabled, ShouldBeTrue)

		disabled, err := parseOptionalBool("false", "filters")
		So(err, ShouldBeNil)
		So(disabled, ShouldBeFalse)
	})

	Convey("筛选开关应该拒绝无效值", t, func() {
		_, err := parseOptionalBool("sometimes", "filters")
		So(err, ShouldNotBeNil)
	})
}

func TestChatMessageRouteRegistration(t *testing.T) {
	Convey("聊天消息路由应该能与现有群组子路由同时注册", t, func() {
		router := (&Router{}).Handler()

		So(router, ShouldNotBeNil)
	})
}

func TestLocalDateRange(t *testing.T) {
	Convey("自然日边界应该使用系统时区而不是服务器本地时区", t, func() {
		location, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)

		start, end, err := localDateRange("2026-07-14", location)

		So(err, ShouldBeNil)
		So(start, ShouldResemble, time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC))
		So(end, ShouldResemble, time.Date(2026, 7, 14, 16, 0, 0, 0, time.UTC))
	})

	Convey("不存在的日期应该被拒绝", t, func() {
		_, _, err := localDateRange("2026-02-30", time.UTC)

		So(err, ShouldNotBeNil)
	})
}

func TestBuildReplyPreview(t *testing.T) {
	Convey("数据库中不存在的回复目标应该返回缺失标记", t, func() {
		preview := buildReplyPreview(99, map[int]model.Message{})

		So(preview.Found, ShouldBeFalse)
		So(preview.TelegramMessageID, ShouldEqual, 99)
	})
}
