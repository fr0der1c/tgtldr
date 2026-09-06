package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/dailydigest"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/jackc/pgx/v5"
)

// handleDailyDigests 返回每日总览历史列表。
func (r *Router) handleDailyDigests(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	page, err := positiveQueryInt(req, "page", 1)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	pageSize, err := positiveQueryInt(req, "pageSize", 20)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := r.dailyDigests.List(req.Context(), page, pageSize)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// handleDailyDigestByID 返回详情，或执行重建与重新投递操作。
func (r *Router) handleDailyDigestByID(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/daily-digests/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		httpx.Error(w, http.StatusNotFound, "not found")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid daily digest id")
		return
	}
	if len(parts) == 1 && req.Method == http.MethodGet {
		item, err := r.dailyDigests.Get(req.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "Daily Digest not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "rerun" && req.Method == http.MethodPost {
		if err := r.dailyDigests.Rerun(req.Context(), id); err != nil {
			if errors.Is(err, dailydigest.ErrBotUnavailable) {
				httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请先启用并配置 Telegram Bot。", "Enable and configure Telegram Bot first."))
				return
			}
			if errors.Is(err, dailydigest.ErrSourcesNotReady) {
				message := strings.TrimSuffix(err.Error(), ": "+dailydigest.ErrSourcesNotReady.Error())
				if err == dailydigest.ErrSourcesNotReady {
					message = r.localized(req.Context(), "总览或来源摘要仍在处理中，请稍后重试。", "The digest or its source summaries are still processing. Try again later.")
				}
				httpx.Error(w, http.StatusBadRequest, message)
				return
			}
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.JSON(w, http.StatusAccepted, map[string]string{"message": "Daily Digest regeneration started"})
		return
	}
	if len(parts) == 2 && parts[1] == "retry-delivery" && req.Method == http.MethodPost {
		if err := r.dailyDigests.RetryDelivery(req.Context(), id); err != nil {
			if errors.Is(err, dailydigest.ErrBotUnavailable) {
				httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请先完成 Telegram Bot 配置。", "Configure Telegram Bot before retrying delivery."))
				return
			}
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"message": "Daily Digest delivery retried"})
		return
	}
	httpx.Error(w, http.StatusNotFound, "not found")
}
