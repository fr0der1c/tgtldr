package store

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildMessageSearchWhere(t *testing.T) {
	Convey("消息搜索应该限定群组并对多关键词使用 AND", t, func() {
		where, args := buildMessageSearchWhere(7, []string{"机场", "订阅"}, MessageDisplayFilter{})

		So(where, ShouldContainSubstring, "m.chat_id = $1")
		So(where, ShouldContainSubstring, "ilike $2")
		So(where, ShouldContainSubstring, "ilike $3")
		So(args, ShouldResemble, []any{int64(7), "%机场%", "%订阅%"})
	})

	Convey("启用群组过滤时应该排除匹配的发言人和关键词", t, func() {
		where, args := buildMessageSearchWhere(7, []string{"机场"}, MessageDisplayFilter{
			Senders: []string{"@Alice"}, Keywords: []string{"验证码"},
		})

		So(where, ShouldContainSubstring, "sender_name")
		So(where, ShouldContainSubstring, "sender_username")
		So(where, ShouldContainSubstring, "like any")
		So(args, ShouldResemble, []any{
			int64(7), "%机场%", []string{"@alice"}, []string{"%验证码%"},
		})
	})
}

func TestMessageSearchMatch(t *testing.T) {
	Convey("正文命中应该返回内容片段", t, func() {
		snippet, fields := messageSearchMatch(model.Message{
			TextContent: "这个机场的订阅地址已经更新，请重新导入。",
			SenderName:  "测试用户",
		}, []string{"机场", "订阅"})

		So(snippet, ShouldContainSubstring, "机场")
		So(snippet, ShouldContainSubstring, "订阅")
		So(fields, ShouldResemble, []string{"content"})
	})

	Convey("发言人命中时应该标记 sender 字段", t, func() {
		_, fields := messageSearchMatch(model.Message{
			SenderName: "测试用户", SenderUsername: "tester",
		}, []string{"tester"})

		So(fields, ShouldResemble, []string{"sender"})
	})
}

func TestNormalizeMessageKeywordValues(t *testing.T) {
	Convey("关键词中的 LIKE 通配符应该按普通字符处理", t, func() {
		values := normalizeMessageKeywordValues([]string{" 50%_OFF "})

		So(values, ShouldResemble, []string{`%50\%\_off%`})
	})
}
