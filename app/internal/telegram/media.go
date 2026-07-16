package telegram

import (
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	messagepeer "github.com/gotd/td/telegram/message/peer"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
)

type storedMessageJSON struct {
	Media *storedMediaJSON `json:"Media"`
}

type storedMediaJSON struct {
	Round    bool                `json:"Round"`
	Video    bool                `json:"Video"`
	Voice    bool                `json:"Voice"`
	Photo    *storedPhotoJSON    `json:"Photo"`
	Document *storedDocumentJSON `json:"Document"`
}

type storedPhotoJSON struct {
	ID            int64             `json:"ID"`
	AccessHash    int64             `json:"AccessHash"`
	FileReference []byte            `json:"FileReference"`
	Sizes         []storedPhotoSize `json:"Sizes"`
}

type storedPhotoSize struct {
	Type string `json:"Type"`
	W    int    `json:"W"`
	H    int    `json:"H"`
}

type storedDocumentJSON struct {
	ID            int64             `json:"ID"`
	AccessHash    int64             `json:"AccessHash"`
	FileReference []byte            `json:"FileReference"`
	Size          int64             `json:"Size"`
	MIMEType      string            `json:"MimeType"`
	Attributes    []json.RawMessage `json:"Attributes"`
}

type sizedPhoto interface {
	GetType() string
	GetW() int
	GetH() int
}

// messageMediaAsset 把 Telegram 媒体转换为持久下载记录，特殊贴纸和动画返回 false。
func messageMediaAsset(accountID int64, msg *tg.Message) (model.MediaAsset, bool) {
	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.AsNotEmpty()
		if !ok {
			return model.MediaAsset{}, false
		}
		thumb := largestPhotoSize(photo.Sizes)
		if thumb == "" {
			return model.MediaAsset{}, false
		}
		return model.MediaAsset{
			TelegramAccountID: accountID, Kind: "photo", MIMEType: "image/jpeg",
			FileName: fmt.Sprintf("photo-%d.jpg", photo.ID), LocationType: "photo",
			TelegramFileID: photo.ID, TelegramAccessHash: photo.AccessHash,
			FileReference: photo.FileReference, ThumbSize: thumb,
		}, true
	case *tg.MessageMediaDocument:
		document, ok := media.Document.AsNotEmpty()
		if !ok || skipDocument(document, media) {
			return model.MediaAsset{}, false
		}
		kind := documentKind(document, media)
		return model.MediaAsset{
			TelegramAccountID: accountID, Kind: kind, MIMEType: document.MimeType,
			FileName: documentFileName(document), FileSize: document.Size,
			LocationType: "document", TelegramFileID: document.ID,
			TelegramAccessHash: document.AccessHash, FileReference: document.FileReference,
		}, true
	default:
		return model.MediaAsset{}, false
	}
}

// storedMediaAsset 从升级前保存的 JSON 中恢复媒体引用，不依赖接口类型反序列化。
func storedMediaAsset(accountID int64, rawJSON string) (model.MediaAsset, bool, error) {
	var message storedMessageJSON
	if err := json.Unmarshal([]byte(rawJSON), &message); err != nil {
		return model.MediaAsset{}, false, fmt.Errorf("decode stored media json: %w", err)
	}
	if message.Media == nil {
		return model.MediaAsset{}, false, nil
	}
	if message.Media.Photo != nil {
		photo := message.Media.Photo
		thumb := largestStoredPhotoSize(photo.Sizes)
		if thumb == "" {
			return model.MediaAsset{}, false, nil
		}
		return model.MediaAsset{
			TelegramAccountID: accountID, Kind: "photo", MIMEType: "image/jpeg",
			FileName: fmt.Sprintf("photo-%d.jpg", photo.ID), LocationType: "photo",
			TelegramFileID: photo.ID, TelegramAccessHash: photo.AccessHash,
			FileReference: photo.FileReference, ThumbSize: thumb,
		}, true, nil
	}
	if message.Media.Document == nil || skipStoredDocument(message.Media) {
		return model.MediaAsset{}, false, nil
	}
	document := message.Media.Document
	return model.MediaAsset{
		TelegramAccountID: accountID, Kind: storedDocumentKind(message.Media),
		MIMEType: document.MIMEType, FileName: storedDocumentFileName(document),
		FileSize: document.Size, LocationType: "document", TelegramFileID: document.ID,
		TelegramAccessHash: document.AccessHash, FileReference: document.FileReference,
	}, true, nil
}

// largestStoredPhotoSize 选择历史 JSON 中面积最大的可下载图片尺寸。
func largestStoredPhotoSize(sizes []storedPhotoSize) string {
	bestType := ""
	bestArea := 0
	for _, size := range sizes {
		area := size.W * size.H
		if area <= bestArea {
			continue
		}
		bestArea = area
		bestType = size.Type
	}
	return bestType
}

// skipStoredDocument 通过稳定字段识别历史贴纸、GIF 和视频圆圈。
func skipStoredDocument(media *storedMediaJSON) bool {
	if media.Round || strings.EqualFold(media.Document.MIMEType, "image/gif") {
		return true
	}
	for _, raw := range media.Document.Attributes {
		var attribute map[string]json.RawMessage
		if json.Unmarshal(raw, &attribute) != nil {
			continue
		}
		if _, sticker := attribute["Stickerset"]; sticker {
			return true
		}
	}
	return false
}

// storedDocumentKind 根据历史消息顶层标记和 MIME 类型归一媒体类型。
func storedDocumentKind(media *storedMediaJSON) string {
	if media.Voice {
		return "voice"
	}
	if media.Video || strings.HasPrefix(media.Document.MIMEType, "video/") {
		return "video"
	}
	if strings.HasPrefix(media.Document.MIMEType, "audio/") {
		return "audio"
	}
	return "document"
}

// storedDocumentFileName 从历史属性中恢复文件名，缺失时生成稳定名称。
func storedDocumentFileName(document *storedDocumentJSON) string {
	for _, raw := range document.Attributes {
		var attribute struct {
			FileName string `json:"FileName"`
		}
		if json.Unmarshal(raw, &attribute) == nil && strings.TrimSpace(attribute.FileName) != "" {
			return sanitizeFileName(attribute.FileName)
		}
	}
	extension := ""
	if extensions, _ := mime.ExtensionsByType(document.MIMEType); len(extensions) > 0 {
		extension = extensions[0]
	}
	return fmt.Sprintf("file-%d%s", document.ID, extension)
}

// largestPhotoSize 按像素面积确定图片消息的最大可用尺寸。
func largestPhotoSize(sizes []tg.PhotoSizeClass) string {
	bestType := ""
	bestArea := 0
	for _, size := range sizes {
		sized, ok := size.(sizedPhoto)
		if !ok {
			continue
		}
		area := sized.GetW() * sized.GetH()
		if area <= bestArea {
			continue
		}
		bestArea = area
		bestType = sized.GetType()
	}
	return bestType
}

// skipDocument 排除第一版不支持的动画、贴纸、自定义表情和视频圆圈。
func skipDocument(document *tg.Document, media *tg.MessageMediaDocument) bool {
	if media.Round || strings.EqualFold(document.MimeType, "image/gif") {
		return true
	}
	for _, attribute := range document.Attributes {
		switch attribute.(type) {
		case *tg.DocumentAttributeAnimated, *tg.DocumentAttributeSticker, *tg.DocumentAttributeCustomEmoji:
			return true
		}
	}
	return false
}

// documentKind 将 Telegram 文档属性归一为网页支持的媒体类型。
func documentKind(document *tg.Document, media *tg.MessageMediaDocument) string {
	if media.Voice {
		return "voice"
	}
	if media.Video || strings.HasPrefix(document.MimeType, "video/") {
		return "video"
	}
	if strings.HasPrefix(document.MimeType, "audio/") {
		return "audio"
	}
	return "document"
}

// documentFileName 优先使用 Telegram 文件名，否则根据 MIME 生成稳定名称。
func documentFileName(document *tg.Document) string {
	for _, attribute := range document.Attributes {
		filename, ok := attribute.(*tg.DocumentAttributeFilename)
		if ok && strings.TrimSpace(filename.FileName) != "" {
			return sanitizeFileName(filename.FileName)
		}
	}
	extension := ""
	if extensions, _ := mime.ExtensionsByType(document.MimeType); len(extensions) > 0 {
		extension = extensions[0]
	}
	return fmt.Sprintf("file-%d%s", document.ID, extension)
}

// sanitizeFileName 去除路径和控制字符，防止文件逃逸目标目录。
func sanitizeFileName(value string) string {
	name := filepath.Base(strings.TrimSpace(value))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." {
		return "file"
	}
	return name
}

// senderEntityFromUpdate 提取实时更新中的发言人实体和头像引用。
func senderEntityFromUpdate(accountID int64, msg *tg.Message, entities tg.Entities) (model.TelegramEntity, bool) {
	peers := messagepeer.EntitiesFromUpdate(entities)
	switch from := msg.FromID.(type) {
	case *tg.PeerUser:
		user, ok := peers.User(from.UserID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return userEntity(accountID, user), true
	case *tg.PeerChannel:
		channel, ok := peers.Channel(from.ChannelID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return channelEntity(accountID, channel), true
	case *tg.PeerChat:
		chat, ok := peers.Chat(from.ChatID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return chatEntity(accountID, chat), true
	default:
		return model.TelegramEntity{}, false
	}
}

// senderEntityFromHistory 提取历史回补结果中的发言人实体和头像引用。
func senderEntityFromHistory(accountID int64, msg *tg.Message, entities messagepeer.Entities) (model.TelegramEntity, bool) {
	switch from := msg.FromID.(type) {
	case *tg.PeerUser:
		user, ok := entities.User(from.UserID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return userEntity(accountID, user), true
	case *tg.PeerChannel:
		channel, ok := entities.Channel(from.ChannelID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return channelEntity(accountID, channel), true
	case *tg.PeerChat:
		chat, ok := entities.Chat(from.ChatID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return chatEntity(accountID, chat), true
	default:
		return model.TelegramEntity{}, false
	}
}

// dialogEntity 从群组同步结果中提取群头像所需实体。
func dialogEntity(accountID int64, elem dialogsquery.Elem) (model.TelegramEntity, bool) {
	switch peer := elem.Peer.(type) {
	case *tg.InputPeerChat:
		chat, ok := elem.Entities.Chat(peer.ChatID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return chatEntity(accountID, chat), true
	case *tg.InputPeerChannel:
		channel, ok := elem.Entities.Channel(peer.ChannelID)
		if !ok {
			return model.TelegramEntity{}, false
		}
		return channelEntity(accountID, channel), true
	default:
		return model.TelegramEntity{}, false
	}
}

// userEntity 归一化用户实体及当前头像 ID。
func userEntity(accountID int64, user *tg.User) model.TelegramEntity {
	return model.TelegramEntity{
		TelegramAccountID: accountID, PeerType: "user", TelegramID: user.ID,
		AccessHash: user.AccessHash, DisplayName: strings.TrimSpace(user.FirstName + " " + user.LastName),
		Username: user.Username, CurrentPhotoID: userPhotoID(user.Photo),
	}
}

// channelEntity 归一化超级群组或频道实体及当前头像 ID。
func channelEntity(accountID int64, channel *tg.Channel) model.TelegramEntity {
	return model.TelegramEntity{
		TelegramAccountID: accountID, PeerType: "channel", TelegramID: channel.ID,
		AccessHash: channel.AccessHash, DisplayName: channel.Title, Username: channel.Username,
		CurrentPhotoID: chatPhotoID(channel.Photo),
	}
}

// chatEntity 归一化基础群组实体及当前头像 ID。
func chatEntity(accountID int64, chat *tg.Chat) model.TelegramEntity {
	return model.TelegramEntity{
		TelegramAccountID: accountID, PeerType: "chat", TelegramID: chat.ID,
		DisplayName: chat.Title, CurrentPhotoID: chatPhotoID(chat.Photo),
	}
}

// userPhotoID 安全读取非空用户头像 ID。
func userPhotoID(photo tg.UserProfilePhotoClass) int64 {
	value, ok := photo.(*tg.UserProfilePhoto)
	if !ok {
		return 0
	}
	return value.PhotoID
}

// chatPhotoID 安全读取非空群组头像 ID。
func chatPhotoID(photo tg.ChatPhotoClass) int64 {
	value, ok := photo.(*tg.ChatPhoto)
	if !ok {
		return 0
	}
	return value.PhotoID
}
