package store

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// TestAppendMessageDisplayFilter 验证消息查看的群组过滤规则会生成完整查询条件。
func TestAppendMessageDisplayFilter(t *testing.T) {
	Convey("关闭机器人消息保留时应该排除机器人消息", t, func() {
		query, args := appendMessageDisplayFilter("m", nil, MessageDisplayFilter{ExcludeBots: true})

		So(query, ShouldContainSubstring, "not m.sender_is_bot")
		So(args, ShouldBeEmpty)
	})

	Convey("机器人、发言人和关键词过滤应该同时生效", t, func() {
		query, args := appendMessageDisplayFilter("m", nil, MessageDisplayFilter{
			ExcludeBots: true,
			Senders:     []string{"Bot User"},
			Keywords:    []string{"推广"},
		})

		So(query, ShouldContainSubstring, "not m.sender_is_bot")
		So(query, ShouldContainSubstring, "sender_name")
		So(query, ShouldContainSubstring, "text_content")
		So(strings.Join(args[0].([]string), ","), ShouldEqual, "bot user")
		So(strings.Join(args[1].([]string), ","), ShouldEqual, "%推广%")
	})
}
