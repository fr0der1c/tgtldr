package api

import (
	"net/http"
	"strings"

	"github.com/frederic/tgtldr/app/internal/httpx"
)

func (r *Router) handleStartHistoryBackfill(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload struct {
		ChatID   int64  `json:"chatId"`
		FromDate string `json:"fromDate"`
		ToDate   string `json:"toDate"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.ChatID == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid chat id")
		return
	}
	if strings.TrimSpace(payload.FromDate) == "" || strings.TrimSpace(payload.ToDate) == "" {
		httpx.Error(w, http.StatusBadRequest, "请填写回补的开始和结束日期。")
		return
	}

	chat, err := r.store.Chats.GetByID(req.Context(), payload.ChatID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err.Error())
		return
	}

	task, err := r.telegram.StartHistoryBackfill(chat, payload.FromDate, payload.ToDate)
	if err != nil {
		if floodErr, ok := asFloodWaitError(err); ok {
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, floodErr.Error(), "telegram_flood_wait", floodErr.RetryAfterSeconds())
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusAccepted, task)
}

func (r *Router) handleHistoryBackfillByID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	taskID := strings.TrimSpace(strings.TrimPrefix(req.URL.Path, "/api/history-backfills/"))
	if taskID == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid task id")
		return
	}

	task, err := r.telegram.GetHistoryBackfillTask(taskID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, task)
}
