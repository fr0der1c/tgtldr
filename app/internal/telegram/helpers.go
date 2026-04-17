package telegram

import (
	"strconv"
	"strings"

	"github.com/frederic/tgtldr/app/internal/model"
	messagepeer "github.com/gotd/td/telegram/message/peer"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
)

func dialogToChat(elem dialogsquery.Elem) (model.Chat, bool) {
	var key dialogsquery.DialogKey
	if err := key.FromInputPeer(elem.Peer); err != nil {
		return model.Chat{}, false
	}

	switch key.Kind {
	case dialogsquery.Chat:
		chat, ok := elem.Entities.Chat(key.ID)
		if !ok {
			return model.Chat{}, false
		}
		return model.Chat{
			TelegramChatID: key.ID,
			TelegramAccess: key.AccessHash,
			Title:          chat.Title,
			ChatType:       "group",
		}, true
	case dialogsquery.Channel:
		channel, ok := elem.Entities.Channel(key.ID)
		if !ok || channel.Broadcast {
			return model.Chat{}, false
		}
		return model.Chat{
			TelegramChatID: key.ID,
			TelegramAccess: channel.AccessHash,
			Title:          channel.Title,
			Username:       channel.Username,
			ChatType:       "supergroup",
		}, true
	default:
		return model.Chat{}, false
	}
}

func extractChat(peer tg.PeerClass) (id int64, kind string, ok bool) {
	switch p := peer.(type) {
	case *tg.PeerChat:
		return p.ChatID, "group", true
	case *tg.PeerChannel:
		return p.ChannelID, "supergroup", true
	default:
		return 0, "", false
	}
}

func resolveSender(msg *tg.Message, entities tg.Entities) (int64, string, string, bool) {
	ent := messagepeer.EntitiesFromUpdate(entities)
	switch from := msg.FromID.(type) {
	case *tg.PeerUser:
		user, ok := ent.User(from.UserID)
		if !ok {
			return from.UserID, "User " + int64String(from.UserID), "", false
		}
		name := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
		if name == "" {
			name = user.Username
		}
		if name == "" {
			name = "User " + int64String(user.ID)
		}
		return user.ID, name, user.Username, user.Bot
	case *tg.PeerChannel:
		channel, ok := ent.Channel(from.ChannelID)
		if ok {
			return channel.ID, channel.Title, channel.Username, false
		}
		return from.ChannelID, "Channel " + int64String(from.ChannelID), "", false
	case *tg.PeerChat:
		chat, ok := ent.Chat(from.ChatID)
		if ok {
			return chat.ID, chat.Title, "", false
		}
		return from.ChatID, "Chat " + int64String(from.ChatID), "", false
	default:
		return 0, "Unknown", "", false
	}
}

func extractCaption(msg *tg.Message) string {
	if msg.Message == "" {
		return ""
	}
	if msg.Media == nil {
		return ""
	}
	return msg.Message
}

func classifyMessage(msg *tg.Message) string {
	if msg.Media == nil {
		return "text"
	}
	return "media"
}

func mediaKind(msg *tg.Message) string {
	switch msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		return "document"
	default:
		if msg.Media == nil {
			return ""
		}
		return "other"
	}
}

func replyToID(msg *tg.Message) int {
	if msg.ReplyTo == nil {
		return 0
	}
	switch reply := msg.ReplyTo.(type) {
	case *tg.MessageReplyHeader:
		return reply.ReplyToMsgID
	default:
		return 0
	}
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
