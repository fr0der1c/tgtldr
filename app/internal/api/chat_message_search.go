package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

type chatMessageSearchResponse struct {
	Items    []chatMessageSearchItem `json:"items"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
}

type chatMessageSearchItem struct {
	chatMessageItem
	LocalDate     string   `json:"localDate"`
	MatchSnippet  string   `json:"matchSnippet"`
	MatchedFields []string `json:"matchedFields"`
}

func (r *Router) handleChatMessageSearch(w http.ResponseWriter, req *http.Request) {
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
	filtersApplied, err := parseOptionalBool(req.URL.Query().Get("filters"), "filters")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	query := strings.TrimSpace(req.URL.Query().Get("q"))
	if query == "" {
		httpx.Error(w, http.StatusBadRequest, "search query is required")
		return
	}
	page, err := parsePositiveInt(req.URL.Query().Get("page"), 1, 0, "page")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	pageSize, err := parsePositiveInt(req.URL.Query().Get("pageSize"), 50, 100, "pageSize")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	_, timezone, err := r.chatMessageActivityWindow(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	result, err := r.store.Messages.Search(req.Context(), store.MessageSearchParams{
		ChatID: chatID, Query: query, Timezone: timezone, Page: page, PageSize: pageSize,
		Filter: chatMessageDisplayFilter(chat, filtersApplied),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]chatMessageSearchItem, 0, len(result.Items))
	for _, resultItem := range result.Items {
		message := resultItem.Message
		items = append(items, chatMessageSearchItem{
			chatMessageItem: chatMessageItem{
				ID: message.ID, TelegramMessageID: message.TelegramMessageID,
				SenderName: message.SenderName, SenderUsername: message.SenderUsername,
				SenderIsBot: message.SenderIsBot, TextContent: message.TextContent,
				Caption: message.Caption, MessageType: message.MessageType,
				MediaKind: message.MediaKind, MessageTime: message.MessageTime,
			},
			LocalDate: resultItem.LocalDate, MatchSnippet: resultItem.MatchSnippet,
			MatchedFields: resultItem.MatchedFields,
		})
	}
	httpx.JSON(w, http.StatusOK, chatMessageSearchResponse{
		Items: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize,
	})
}

func parsePositiveInt(value string, fallback int, max int, name string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || max > 0 && parsed > max {
		return 0, errors.New("invalid " + name)
	}
	return parsed, nil
}
