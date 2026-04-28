package summary

import (
	"strings"
	"testing"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSplitMessages(t *testing.T) {
	Convey("长时间间隔不会把过小的上下文提前切碎", t, func() {
		base := time.Date(2026, 4, 16, 9, 0, 0, 0, time.Local)
		messages := []model.Message{
			{TelegramMessageID: 1, TextContent: repeatedText("a", 160), MessageTime: base},
			{TelegramMessageID: 2, TextContent: repeatedText("b", 160), MessageTime: base.Add(2 * time.Hour)},
		}

		chunks := SplitMessages(messages, 1000)

		So(chunks, ShouldHaveLength, 1)
		So(chunks[0].Messages, ShouldHaveLength, 2)
	})

	Convey("长时间间隔会在 chunk 已经足够大时优先切块", t, func() {
		base := time.Date(2026, 4, 16, 9, 0, 0, 0, time.Local)
		messages := []model.Message{
			{TelegramMessageID: 1, TextContent: repeatedText("a", 1500), MessageTime: base},
			{TelegramMessageID: 2, TextContent: repeatedText("b", 1500), MessageTime: base.Add(2 * time.Hour)},
		}

		chunks := SplitMessages(messages, 1000)

		So(chunks, ShouldHaveLength, 2)
		So(chunks[0].Messages, ShouldHaveLength, 1)
		So(chunks[1].Messages, ShouldHaveLength, 1)
	})
}

func TestBuildTranscriptReferenceFallback(t *testing.T) {
	Convey("引用无文本媒体消息时应该输出明确占位", t, func() {
		base := time.Date(2026, 4, 16, 9, 0, 0, 0, time.Local)
		reference := model.Message{
			TelegramMessageID: 100,
			SenderName:        "Alice",
			MessageTime:       base,
			MessageType:       "media",
			MediaKind:         "photo",
		}
		reply := model.Message{
			TelegramMessageID: 101,
			SenderName:        "Bob",
			MessageTime:       base.Add(time.Minute),
			TextContent:       "看到了",
			ReplyToMessageID:  100,
		}

		transcript := BuildTranscript(
			[]model.Message{reply},
			map[int]model.Message{
				100: reference,
				101: reply,
			},
			time.Local,
			model.LanguageZhCN,
		)

		So(transcript, ShouldContainSubstring, "[Referenced Messages]")
		So(transcript, ShouldContainSubstring, "[图片消息，无文字说明]")
		So(transcript, ShouldContainSubstring, `reply_excerpt="[图片消息，无文字说明]"`)
	})

	Convey("找不到原始引用消息时应该明确说明", t, func() {
		base := time.Date(2026, 4, 16, 9, 0, 0, 0, time.Local)
		reply := model.Message{
			TelegramMessageID: 101,
			SenderName:        "Bob",
			MessageTime:       base,
			TextContent:       "看到了",
			ReplyToMessageID:  999,
		}

		transcript := BuildTranscript(
			[]model.Message{reply},
			map[int]model.Message{
				101: reply,
			},
			time.Local,
			model.LanguageZhCN,
		)

		So(transcript, ShouldContainSubstring, "reply_to=[msg:999]")
		So(transcript, ShouldContainSubstring, `reply_excerpt="[原始消息未在当前数据库中找到]"`)
	})

	Convey("英文语言下引用占位也使用英文", t, func() {
		base := time.Date(2026, 4, 16, 9, 0, 0, 0, time.Local)
		reply := model.Message{
			TelegramMessageID: 101,
			SenderName:        "Bob",
			MessageTime:       base,
			TextContent:       "saw it",
			ReplyToMessageID:  999,
		}

		transcript := BuildTranscript(
			[]model.Message{reply},
			map[int]model.Message{101: reply},
			time.Local,
			model.LanguageEN,
		)

		So(transcript, ShouldContainSubstring, `reply_excerpt="[Original message was not found in the current database]"`)
	})
}

func repeatedText(token string, length int) string {
	return strings.Repeat(token, length)
}
