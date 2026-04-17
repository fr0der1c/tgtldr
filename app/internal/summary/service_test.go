package summary

import (
	"testing"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPrepareMessages(t *testing.T) {
	Convey("显式过滤规则会在生成摘要前生效", t, func() {
		base := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
		messages := []model.Message{
			{
				TelegramMessageID: 1,
				SenderName:        "验证机器人",
				SenderUsername:    "verify_bot",
				SenderIsBot:       true,
				TextContent:       "请完成入群验证",
				MessageTime:       base,
			},
			{
				TelegramMessageID: 2,
				SenderName:        "Alice",
				SenderUsername:    "alice",
				TextContent:       "正常消息",
				MessageTime:       base.Add(time.Minute),
			},
			{
				TelegramMessageID: 3,
				SenderName:        "Bob",
				SenderUsername:    "bob",
				TextContent:       "包含敏感词 验证码",
				MessageTime:       base.Add(2 * time.Minute),
			},
		}

		chat := model.Chat{
			KeepBotMessages:  false,
			FilteredSenders:  []string{"@alice"},
			FilteredKeywords: []string{"验证码"},
		}

		service := &Service{}
		filtered, lookup, err := service.prepareMessages(t.Context(), chat, messages)
		So(err, ShouldBeNil)
		So(len(filtered), ShouldEqual, 0)
		So(len(lookup), ShouldEqual, 3)
	})
}
