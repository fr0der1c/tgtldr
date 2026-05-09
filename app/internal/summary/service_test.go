package summary

import (
	"errors"
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

func TestOpenAIErrorContext(t *testing.T) {
	Convey("OpenAI 请求 context 会记录复现所需参数且不包含密钥", t, func() {
		chunkIndex := 0
		snapshot := buildOpenAIRequestSnapshot(openAIRequestContextInput{
			Stage:        "chunk",
			ChunkIndex:   &chunkIndex,
			BaseURL:      "https://example.com/v1",
			Model:        "gpt-test",
			Temperature:  0.2,
			MaxOutput:    16,
			SystemPrompt: "system prompt",
			UserPrompt:   "user prompt",
		})

		So(snapshot.Context, ShouldContainSubstring, `"stage": "chunk"`)
		So(snapshot.Context, ShouldContainSubstring, `"chunkIndex": 0`)
		So(snapshot.Context, ShouldContainSubstring, `"baseURL": "https://example.com/v1"`)
		So(snapshot.Context, ShouldContainSubstring, `"model": "gpt-test"`)
		So(snapshot.Context, ShouldNotContainSubstring, "systemPrompt")
		So(snapshot.Context, ShouldNotContainSubstring, "userPrompt")
		So(snapshot.Context, ShouldNotContainSubstring, "apiKey")
		So(snapshot.Context, ShouldNotContainSubstring, "Authorization")
		So(snapshot.SystemPrompt, ShouldEqual, "system prompt")
		So(snapshot.UserPrompt, ShouldEqual, "user prompt")
	})

	Convey("OpenAI 错误包装会保留原错误文案并带出请求 context", t, func() {
		wrapped := wrapOpenAIRequestError(
			errors.New("openai status 504: error code: 504"),
			openAIRequestSnapshot{
				Context:      "request context",
				SystemPrompt: "system prompt",
				UserPrompt:   "user prompt",
			},
		)

		So(wrapped.Error(), ShouldEqual, "openai status 504: error code: 504")
		So(openAIErrorContext(wrapped), ShouldEqual, "request context")
		So(openAIErrorSystemPrompt(wrapped), ShouldEqual, "system prompt")
		So(openAIErrorUserPrompt(wrapped), ShouldEqual, "user prompt")
	})
}
