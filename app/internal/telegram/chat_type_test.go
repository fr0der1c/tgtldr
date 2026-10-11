package telegram

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	messagepeer "github.com/gotd/td/telegram/message/peer"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
	. "github.com/smartystreets/goconvey/convey"
)

func channelDialog(channel *tg.Channel) dialogsquery.Elem {
	return dialogsquery.Elem{
		Peer: &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash},
		Entities: messagepeer.NewEntities(nil, nil, map[int64]*tg.Channel{
			channel.ID: channel,
		}),
	}
}

func TestDialogToChatChannelTypes(t *testing.T) {
	Convey("广播频道会被识别为 channel 类型", t, func() {
		elem := channelDialog(&tg.Channel{ID: 101, AccessHash: 9001, Title: "News", Username: "news", Broadcast: true})

		chat, ok := dialogToChat(elem)

		So(ok, ShouldBeTrue)
		So(chat.ChatType, ShouldEqual, model.ChatTypeChannel)
		So(chat.TelegramChatID, ShouldEqual, 101)
		So(chat.TelegramAccess, ShouldEqual, 9001)
		So(chat.Title, ShouldEqual, "News")
	})

	Convey("超级群组仍然识别为 supergroup 类型", t, func() {
		elem := channelDialog(&tg.Channel{ID: 202, AccessHash: 9002, Title: "Group", Megagroup: true})

		chat, ok := dialogToChat(elem)

		So(ok, ShouldBeTrue)
		So(chat.ChatType, ShouldEqual, model.ChatTypeSupergroup)
	})
}

func TestInputPeerForChannel(t *testing.T) {
	Convey("频道历史回补使用 InputPeerChannel", t, func() {
		peer, err := inputPeerForChat(model.Chat{ChatType: model.ChatTypeChannel, TelegramChatID: 101, TelegramAccess: 9001})

		So(err, ShouldBeNil)
		channelPeer, ok := peer.(*tg.InputPeerChannel)
		So(ok, ShouldBeTrue)
		So(channelPeer.ChannelID, ShouldEqual, 101)
		So(channelPeer.AccessHash, ShouldEqual, 9001)
	})
}

func TestUsesChannelPeer(t *testing.T) {
	Convey("超级群组和频道都通过 Channel 接口访问，普通群组不是", t, func() {
		So(model.UsesChannelPeer(model.ChatTypeSupergroup), ShouldBeTrue)
		So(model.UsesChannelPeer(model.ChatTypeChannel), ShouldBeTrue)
		So(model.UsesChannelPeer(model.ChatTypeGroup), ShouldBeFalse)
	})
}
