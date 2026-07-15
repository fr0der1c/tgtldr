package store

import (
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestReverseMessages(t *testing.T) {
	Convey("倒序查询的消息应该恢复为稳定的时间正序", t, func() {
		base := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		messages := []model.Message{
			{TelegramMessageID: 3, MessageTime: base.Add(time.Minute)},
			{TelegramMessageID: 2, MessageTime: base},
			{TelegramMessageID: 1, MessageTime: base},
		}

		reverseMessages(messages)

		So(messages[0].TelegramMessageID, ShouldEqual, 1)
		So(messages[1].TelegramMessageID, ShouldEqual, 2)
		So(messages[2].TelegramMessageID, ShouldEqual, 3)
	})

	Convey("2001 条倒序结果应该只返回最新 2000 条并标记仍有更早消息", t, func() {
		messages := make([]model.Message, 2001)
		for index := range messages {
			messages[index].TelegramMessageID = 2001 - index
		}

		page, hasMore := normalizeMessagePage(messages, 2000)

		So(hasMore, ShouldBeTrue)
		So(len(page), ShouldEqual, 2000)
		So(page[0].TelegramMessageID, ShouldEqual, 2)
		So(page[1999].TelegramMessageID, ShouldEqual, 2001)
	})
}
