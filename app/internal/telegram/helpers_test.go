package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
	. "github.com/smartystreets/goconvey/convey"
)

// TestResolveSenderFallback 验证匿名管理员消息使用群组公开身份兜底。
func TestResolveSenderFallback(t *testing.T) {
	Convey("from_id 缺失时使用消息所属群组", t, func() {
		message := &tg.Message{PeerID: &tg.PeerChannel{ChannelID: 42}}

		senderID, name, username, isBot := resolveSender(message, tg.Entities{}, "测试群")

		So(senderID, ShouldEqual, int64(42))
		So(name, ShouldEqual, "测试群")
		So(username, ShouldBeEmpty)
		So(isBot, ShouldBeFalse)
	})
}
