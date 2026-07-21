package telegram

import (
	"strconv"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/model"
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

// resolveSender 从实时更新实体解析公开发送身份，并用当前群组信息兜底。
func resolveSender(msg *tg.Message, entities tg.Entities, chatTitle string) (int64, string, string, bool) {
	ent := messagepeer.EntitiesFromUpdate(entities)
	switch from := messageSenderPeer(msg).(type) {
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
		if strings.TrimSpace(chatTitle) != "" && samePeer(msg.PeerID, from) {
			return from.ChannelID, chatTitle, "", false
		}
		return from.ChannelID, "Channel " + int64String(from.ChannelID), "", false
	case *tg.PeerChat:
		chat, ok := ent.Chat(from.ChatID)
		if ok {
			return chat.ID, chat.Title, "", false
		}
		if strings.TrimSpace(chatTitle) != "" && samePeer(msg.PeerID, from) {
			return from.ChatID, chatTitle, "", false
		}
		return from.ChatID, "Chat " + int64String(from.ChatID), "", false
	default:
		return 0, fallback(chatTitle, "Unknown"), "", false
	}
}

// messageSenderPeer 在 Telegram 省略 from_id 时，将会话本身视为公开发送身份。
func messageSenderPeer(msg *tg.Message) tg.PeerClass {
	if msg.FromID != nil {
		return msg.FromID
	}
	return msg.PeerID
}

// samePeer 比较 Telegram peer 的类型和 ID，避免不同实体空间的数字 ID 混淆。
func samePeer(left tg.PeerClass, right tg.PeerClass) bool {
	switch leftPeer := left.(type) {
	case *tg.PeerChannel:
		rightPeer, ok := right.(*tg.PeerChannel)
		return ok && leftPeer.ChannelID == rightPeer.ChannelID
	case *tg.PeerChat:
		rightPeer, ok := right.(*tg.PeerChat)
		return ok && leftPeer.ChatID == rightPeer.ChatID
	case *tg.PeerUser:
		rightPeer, ok := right.(*tg.PeerUser)
		return ok && leftPeer.UserID == rightPeer.UserID
	default:
		return false
	}
}

func fallback(value string, fallbackValue string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallbackValue
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
	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		if document, ok := media.Document.AsNotEmpty(); ok && isStickerDocument(document) {
			return "sticker"
		}
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
