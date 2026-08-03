package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

type readAPIChat struct {
	ID             int64  `json:"id"`
	TelegramChatID int64  `json:"telegramChatId"`
	Title          string `json:"title"`
	Username       string `json:"username"`
}

type readAPIParticipant struct {
	TelegramSenderID int64     `json:"telegramSenderId"`
	SenderName       string    `json:"senderName"`
	SenderUsername   string    `json:"senderUsername"`
	MessageCount     int64     `json:"messageCount"`
	FirstMessageTime time.Time `json:"firstMessageTime"`
	LastMessageTime  time.Time `json:"lastMessageTime"`
}

type readAPIMessage struct {
	TelegramMessageID int       `json:"telegramMessageId"`
	TelegramSenderID  int64     `json:"telegramSenderId"`
	SenderName        string    `json:"senderName"`
	SenderUsername    string    `json:"senderUsername"`
	TextContent       string    `json:"textContent"`
	Caption           string    `json:"caption"`
	MessageType       string    `json:"messageType"`
	MediaKind         string    `json:"mediaKind"`
	ReplyToMessageID  int       `json:"replyToMessageId"`
	MessageTime       time.Time `json:"messageTime"`
}

type readAPIMessageListResponse struct {
	Items      []readAPIMessage `json:"items"`
	HasMore    bool             `json:"hasMore"`
	NextCursor string           `json:"nextCursor"`
}

func (r *Router) handleReadAPIChats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	chats, err := r.store.Chats.List(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]readAPIChat, 0, len(chats))
	for _, chat := range chats {
		items = append(items, readAPIChat{
			ID:             chat.ID,
			TelegramChatID: chat.TelegramChatID,
			Title:          chat.Title,
			Username:       chat.Username,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (r *Router) handleReadAPIChatResource(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	chatID, resource, ok := parseReadAPIChatPath(req.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "endpoint not found")
		return
	}
	if _, err := r.store.Chats.GetByID(req.Context(), chatID); err != nil {
		if store.IsNotFound(err) {
			httpx.Error(w, http.StatusNotFound, "chat not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch resource {
	case "participants":
		r.handleReadAPIParticipants(w, req, chatID)
	case "messages":
		r.handleReadAPIMessages(w, req, chatID)
	default:
		httpx.Error(w, http.StatusNotFound, "endpoint not found")
	}
}

func (r *Router) handleReadAPIParticipants(w http.ResponseWriter, req *http.Request, chatID int64) {
	query, limit, err := parseParticipantQuery(req.URL.Query())
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	participants, err := r.store.Messages.SearchParticipants(req.Context(), chatID, query, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]readAPIParticipant, 0, len(participants))
	for _, participant := range participants {
		items = append(items, readAPIParticipant(participant))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (r *Router) handleReadAPIMessages(w http.ResponseWriter, req *http.Request, chatID int64) {
	query, err := parseMessageListQuery(req.URL.Query())
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	var cursorTime *time.Time
	var cursorMessageID int
	if query.Cursor != nil {
		cursorTime = &query.Cursor.MessageTime
		cursorMessageID = query.Cursor.TelegramMessageID
	}
	messages, err := r.store.Messages.ListMessages(
		req.Context(), chatID, query.SenderID, query.From, query.To,
		cursorTime, cursorMessageID, query.Limit+1,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	hasMore := len(messages) > query.Limit
	if hasMore {
		messages = messages[:query.Limit]
	}
	items := make([]readAPIMessage, 0, len(messages))
	for _, message := range messages {
		items = append(items, toReadAPIMessage(message))
	}
	nextCursor := ""
	if hasMore && len(messages) > 0 {
		last := messages[len(messages)-1]
		nextCursor = encodeMessageCursor(messageCursor{
			MessageTime:       last.MessageTime,
			TelegramMessageID: last.TelegramMessageID,
		})
	}
	httpx.JSON(w, http.StatusOK, readAPIMessageListResponse{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	})
}

func parseReadAPIChatPath(path string) (int64, string, bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/chats/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", false
	}
	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || chatID <= 0 {
		return 0, "", false
	}
	return chatID, parts[1], true
}

func toReadAPIMessage(message model.Message) readAPIMessage {
	return readAPIMessage{
		TelegramMessageID: message.TelegramMessageID,
		TelegramSenderID:  message.TelegramSenderID,
		SenderName:        message.SenderName,
		SenderUsername:    message.SenderUsername,
		TextContent:       message.TextContent,
		Caption:           message.Caption,
		MessageType:       message.MessageType,
		MediaKind:         message.MediaKind,
		ReplyToMessageID:  message.ReplyToMessageID,
		MessageTime:       message.MessageTime,
	}
}

const (
	defaultReadAPILimit     = 100
	maxReadAPILimit         = 500
	defaultParticipantLimit = 20
	maxParticipantLimit     = 100
)

type messageCursor struct {
	MessageTime       time.Time `json:"time"`
	TelegramMessageID int       `json:"messageId"`
}

type messageListQuery struct {
	SenderID *int64
	From     *time.Time
	To       *time.Time
	Limit    int
	Cursor   *messageCursor
}

func readAPIRequestAuthorized(req *http.Request, configuredToken string) bool {
	if configuredToken == "" {
		return false
	}
	const prefix = "Bearer "
	header := req.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	expectedHash := sha256.Sum256([]byte(configuredToken))
	providedHash := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) == 1
}

func encodeMessageCursor(cursor messageCursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeMessageCursor(raw string) (messageCursor, error) {
	var cursor messageCursor
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor, errors.New("invalid cursor")
	}
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.MessageTime.IsZero() || cursor.TelegramMessageID <= 0 {
		return messageCursor{}, errors.New("invalid cursor")
	}
	return cursor, nil
}

func parseMessageListQuery(values url.Values) (messageListQuery, error) {
	var out messageListQuery
	senderIDRaw := strings.TrimSpace(values.Get("senderId"))
	if senderIDRaw != "" {
		senderID, err := strconv.ParseInt(senderIDRaw, 10, 64)
		if err != nil || senderID <= 0 {
			return out, errors.New("senderId must be a positive integer")
		}
		out.SenderID = &senderID
	}

	var err error
	if out.From, err = parseOptionalRFC3339(values.Get("from"), "from"); err != nil {
		return messageListQuery{}, err
	}
	if out.To, err = parseOptionalRFC3339(values.Get("to"), "to"); err != nil {
		return messageListQuery{}, err
	}
	if out.From != nil && out.To != nil && !out.From.Before(*out.To) {
		return messageListQuery{}, errors.New("from must be before to")
	}
	out.Limit, err = parseBoundedLimit(values.Get("limit"), defaultReadAPILimit, maxReadAPILimit)
	if err != nil {
		return messageListQuery{}, err
	}
	if raw := strings.TrimSpace(values.Get("cursor")); raw != "" {
		cursor, err := decodeMessageCursor(raw)
		if err != nil {
			return messageListQuery{}, err
		}
		out.Cursor = &cursor
	}
	return out, nil
}

func parseParticipantQuery(values url.Values) (string, int, error) {
	query := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(values.Get("query")), "@"))
	if query == "" {
		return "", 0, errors.New("query is required")
	}
	limit, err := parseBoundedLimit(values.Get("limit"), defaultParticipantLimit, maxParticipantLimit)
	if err != nil {
		return "", 0, err
	}
	return query, limit, nil
}

func parseOptionalRFC3339(raw string, name string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339", name)
	}
	return &parsed, nil
}

func parseBoundedLimit(raw string, fallback int, maximum int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if limit > maximum {
		return maximum, nil
	}
	return limit, nil
}
