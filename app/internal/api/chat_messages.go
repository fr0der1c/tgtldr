package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

const (
	defaultChatMessageLimit = 2000
	maxChatMessageLimit     = 2000
	focusedMessageContext   = 100
)

type chatMessageResponse struct {
	Chat              chatMessageChat             `json:"chat"`
	Date              string                      `json:"date"`
	Timezone          string                      `json:"timezone"`
	Total             int                         `json:"total"`
	Messages          []chatMessageItem           `json:"messages"`
	MessageActivity   []model.ChatMessageActivity `json:"messageActivity"`
	PreviousDate      string                      `json:"previousDate,omitempty"`
	NextDate          string                      `json:"nextDate,omitempty"`
	HasMoreBefore     bool                        `json:"hasMoreBefore"`
	BeforeCursor      string                      `json:"beforeCursor,omitempty"`
	FocusedMessageID  int64                       `json:"focusedMessageId,omitempty"`
	HasMessageFilters bool                        `json:"hasMessageFilters"`
	FiltersApplied    bool                        `json:"filtersApplied"`
}

type chatMessageChat struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	Enabled   bool   `json:"enabled"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type chatMessageItem struct {
	ID                int64                    `json:"id"`
	TelegramMessageID int                      `json:"telegramMessageId"`
	SenderName        string                   `json:"senderName"`
	SenderUsername    string                   `json:"senderUsername"`
	SenderIsBot       bool                     `json:"senderIsBot"`
	TextContent       string                   `json:"textContent"`
	Caption           string                   `json:"caption"`
	MessageType       string                   `json:"messageType"`
	MediaKind         string                   `json:"mediaKind"`
	MessageTime       time.Time                `json:"messageTime"`
	Reply             *chatMessageReplyPreview `json:"reply,omitempty"`
	Media             *chatMessageMedia        `json:"media,omitempty"`
	SenderAvatarURL   string                   `json:"senderAvatarUrl,omitempty"`
}

type chatMessageMedia struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	MIMEType    string `json:"mimeType"`
	FileName    string `json:"fileName"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	ContentURL  string `json:"contentUrl,omitempty"`
	Error       string `json:"error,omitempty"`
	CanDownload bool   `json:"canDownload"`
	CanRetry    bool   `json:"canRetry"`
}

type chatMessageReplyPreview struct {
	TelegramMessageID int    `json:"telegramMessageId"`
	Found             bool   `json:"found"`
	SenderName        string `json:"senderName"`
	TextContent       string `json:"textContent"`
	Caption           string `json:"caption"`
	MessageType       string `json:"messageType"`
	MediaKind         string `json:"mediaKind"`
}

type encodedMessageCursor struct {
	MessageTime       string `json:"t"`
	TelegramMessageID int    `json:"id"`
}

func (r *Router) handleChatMessages(w http.ResponseWriter, req *http.Request) {
	chatID, err := strconv.ParseInt(req.PathValue("chatID"), 10, 64)
	if err != nil || chatID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid chat id")
		return
	}
	chat, err := r.store.Chats.GetByID(req.Context(), chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		httpx.Error(w, status, err.Error())
		return
	}

	startActivity, timezone, err := r.chatMessageActivityWindow(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	filtersApplied, err := parseOptionalBool(req.URL.Query().Get("filters"), "filters")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	filter := chatMessageDisplayFilter(chat, filtersApplied)
	date, start, end, err := r.resolveChatMessageDate(req, chatID, timezone, filter)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := parseChatMessageLimit(req.URL.Query().Get("limit"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	cursor, err := decodeMessageCursor(req.URL.Query().Get("before"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	focusMessageID, err := parsePositiveInt64(req.URL.Query().Get("focusMessageId"), "focusMessageId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	messages, hasMore, err := r.listChatMessageWindow(
		req, chatID, start, end, cursor, limit, focusMessageID, filter,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "focused message not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response, err := r.buildChatMessageResponse(
		req, chat, date, timezone, start, end, startActivity, messages, hasMore, filter,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.FocusedMessageID = focusMessageID
	httpx.JSON(w, http.StatusOK, response)
}

func (r *Router) listChatMessageWindow(
	req *http.Request,
	chatID int64,
	start time.Time,
	end time.Time,
	cursor *store.MessageCursor,
	limit int,
	focusMessageID int64,
	filter store.MessageDisplayFilter,
) ([]model.Message, bool, error) {
	if focusMessageID > 0 {
		return r.store.Messages.ListWindowAround(
			req.Context(), chatID, start, end, focusMessageID,
			focusedMessageContext, focusedMessageContext,
			filter,
		)
	}
	return r.store.Messages.ListPageForRange(req.Context(), chatID, start, end, cursor, limit, filter)
}

func (r *Router) resolveChatMessageDate(
	req *http.Request,
	chatID int64,
	timezone string,
	filter store.MessageDisplayFilter,
) (string, time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", time.Time{}, time.Time{}, fmt.Errorf("load timezone %s: %w", timezone, err)
	}
	date := strings.TrimSpace(req.URL.Query().Get("date"))
	if date == "" {
		date = time.Now().In(location).Format("2006-01-02")
		latest, err := r.store.Messages.LatestDateAtOrBefore(
			req.Context(), chatID, timezone, date, filter,
		)
		if err != nil {
			return "", time.Time{}, time.Time{}, err
		}
		if latest != "" {
			date = latest
		}
	}
	start, end, err := localDateRange(date, location)
	return date, start, end, err
}

func localDateRange(date string, location *time.Location) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil || start.Format("2006-01-02") != date {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date")
	}
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func (r *Router) buildChatMessageResponse(
	req *http.Request,
	chat model.Chat,
	date string,
	timezone string,
	start time.Time,
	end time.Time,
	activityStart time.Time,
	messages []model.Message,
	hasMore bool,
	filter store.MessageDisplayFilter,
) (chatMessageResponse, error) {
	total, err := r.store.Messages.CountForRange(req.Context(), chat.ID, start, end, filter)
	if err != nil {
		return chatMessageResponse{}, err
	}
	activity, err := r.store.Messages.ActivityForRange(
		req.Context(), chat.ID, activityStart, chatMessageActivityDays, timezone, filter,
	)
	if err != nil {
		return chatMessageResponse{}, err
	}
	previousDate, nextDate, err := r.store.Messages.AdjacentDates(
		req.Context(), chat.ID, timezone, date, filter,
	)
	if err != nil {
		return chatMessageResponse{}, err
	}
	items, err := r.chatMessageItems(req, chat.ID, messages)
	if err != nil {
		return chatMessageResponse{}, err
	}

	response := chatMessageResponse{
		Chat: chatMessageChat{
			ID: chat.ID, Title: chat.Title, Username: chat.Username, Enabled: chat.Enabled,
		},
		Date:              date,
		Timezone:          timezone,
		Total:             total,
		Messages:          items,
		MessageActivity:   activity,
		PreviousDate:      previousDate,
		NextDate:          nextDate,
		HasMoreBefore:     hasMore,
		HasMessageFilters: hasChatMessageFilters(chat),
		FiltersApplied:    filter.ExcludeBots || len(filter.Senders) > 0 || len(filter.Keywords) > 0,
	}
	peerType := "chat"
	if chat.ChatType == "supergroup" {
		peerType = "channel"
	}
	if avatar, err := r.store.Assets.FindEntityAvatar(req.Context(), chat.CollectorAccountID, peerType, chat.TelegramChatID); err == nil {
		response.Chat.AvatarURL = assetContentURL(avatar.ID)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return chatMessageResponse{}, err
	}
	if hasMore && len(messages) > 0 {
		response.BeforeCursor = encodeMessageCursor(messages[0])
	}
	return response, nil
}

func chatMessageDisplayFilter(chat model.Chat, enabled bool) store.MessageDisplayFilter {
	if !enabled {
		return store.MessageDisplayFilter{}
	}
	return store.MessageDisplayFilter{
		ExcludeBots: !chat.KeepBotMessages,
		Senders:     chat.FilteredSenders,
		Keywords:    chat.FilteredKeywords,
	}
}

func hasChatMessageFilters(chat model.Chat) bool {
	return !chat.KeepBotMessages || len(chat.FilteredSenders) > 0 || len(chat.FilteredKeywords) > 0
}

func parseOptionalBool(value string, name string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "", "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, errors.New("invalid " + name)
	}
}

func (r *Router) chatMessageItems(
	req *http.Request,
	chatID int64,
	messages []model.Message,
) ([]chatMessageItem, error) {
	replyIDs := make([]int, 0)
	for _, message := range messages {
		if message.ReplyToMessageID > 0 {
			replyIDs = append(replyIDs, message.ReplyToMessageID)
		}
	}
	replies, err := r.store.Messages.LookupByTelegramIDs(req.Context(), chatID, replyIDs)
	if err != nil {
		return nil, err
	}
	messageIDs := make([]int64, 0, len(messages))
	for _, message := range messages {
		messageIDs = append(messageIDs, message.ID)
	}
	media, avatars, err := r.store.Assets.ListForMessages(req.Context(), messageIDs)
	if err != nil {
		return nil, err
	}
	items := make([]chatMessageItem, 0, len(messages))
	for _, message := range messages {
		item := chatMessageItem{
			ID: message.ID, TelegramMessageID: message.TelegramMessageID,
			SenderName: message.SenderName, SenderUsername: message.SenderUsername,
			SenderIsBot: message.SenderIsBot, TextContent: message.TextContent,
			Caption: message.Caption, MessageType: message.MessageType,
			MediaKind: message.MediaKind, MessageTime: message.MessageTime,
		}
		if message.ReplyToMessageID > 0 {
			item.Reply = buildReplyPreview(message.ReplyToMessageID, replies)
		}
		if avatar, ok := avatars[message.ID]; ok {
			item.SenderAvatarURL = assetContentURL(avatar.ID)
		}
		if asset, ok := media[message.ID]; ok {
			item.Media = mediaResponse(asset)
		}
		items = append(items, item)
	}
	return items, nil
}

// mediaResponse 只向网页暴露状态和受保护 URL，不返回本地路径或 Telegram 凭据。
func mediaResponse(asset model.MediaAsset) *chatMessageMedia {
	response := &chatMessageMedia{
		ID: asset.ID, Kind: asset.Kind, MIMEType: asset.MIMEType,
		FileName: asset.FileName, Size: asset.FileSize, Status: asset.Status,
		Error: asset.ErrorMessage, CanDownload: asset.Status == "skipped_oversize",
		CanRetry: asset.Status == "failed",
	}
	if asset.Status == "succeeded" {
		response.ContentURL = assetContentURL(asset.ID)
	}
	return response
}

func assetContentURL(id int64) string {
	return fmt.Sprintf("/api/assets/%d/content", id)
}

func buildReplyPreview(
	telegramMessageID int,
	replies map[int]model.Message,
) *chatMessageReplyPreview {
	preview := &chatMessageReplyPreview{TelegramMessageID: telegramMessageID}
	reply, ok := replies[telegramMessageID]
	if !ok {
		return preview
	}
	preview.Found = true
	preview.SenderName = reply.SenderName
	preview.TextContent = reply.TextContent
	preview.Caption = reply.Caption
	preview.MessageType = reply.MessageType
	preview.MediaKind = reply.MediaKind
	return preview
}

func parseChatMessageLimit(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return defaultChatMessageLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > maxChatMessageLimit {
		return 0, fmt.Errorf("invalid limit")
	}
	return limit, nil
}

func parsePositiveInt64(value string, name string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return parsed, nil
}

func encodeMessageCursor(message model.Message) string {
	payload, _ := json.Marshal(encodedMessageCursor{
		MessageTime:       message.MessageTime.UTC().Format(time.RFC3339Nano),
		TelegramMessageID: message.TelegramMessageID,
	})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeMessageCursor(value string) (*store.MessageCursor, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid before cursor")
	}
	var encoded encodedMessageCursor
	if err := json.Unmarshal(payload, &encoded); err != nil {
		return nil, fmt.Errorf("invalid before cursor")
	}
	messageTime, err := time.Parse(time.RFC3339Nano, encoded.MessageTime)
	if err != nil || encoded.TelegramMessageID <= 0 {
		return nil, fmt.Errorf("invalid before cursor")
	}
	return &store.MessageCursor{
		MessageTime: messageTime, TelegramMessageID: encoded.TelegramMessageID,
	}, nil
}
