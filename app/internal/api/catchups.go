package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/catchup"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

// handleCatchUps 路由 Catch Up 创建和历史列表请求。
func (r *Router) handleCatchUps(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listCatchUps(w, req)
	case http.MethodPost:
		r.createCatchUp(w, req)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// createCatchUp 校验用户输入并提交持久化后台任务。
func (r *Router) createCatchUp(w http.ResponseWriter, req *http.Request) {
	var payload struct {
		FromDate       string  `json:"fromDate"`
		ToDate         string  `json:"toDate"`
		ChatIDs        []int64 `json:"chatIds"`
		SendToTelegram bool    `json:"sendToTelegram"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := r.catchups.Start(req.Context(), catchup.CreateInput{
		FromDate: payload.FromDate, ToDate: payload.ToDate, ChatIDs: payload.ChatIDs,
		DeliveryRequested: payload.SendToTelegram,
	})
	if err != nil {
		r.writeCatchUpError(w, req, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, item)
}

// listCatchUps 解析分页参数并返回历史记录。
func (r *Router) listCatchUps(w http.ResponseWriter, req *http.Request) {
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
	items, err := r.catchups.List(req.Context(), page, pageSize)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// handleCatchUpByID 返回详情或重试独立的 Telegram 投递。
func (r *Router) handleCatchUpByID(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/catch-ups/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		httpx.Error(w, http.StatusNotFound, "not found")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid Catch Up id")
		return
	}
	if len(parts) == 1 && req.Method == http.MethodGet {
		item, err := r.catchups.Get(req.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "Catch Up not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "retry-delivery" && req.Method == http.MethodPost {
		if err := r.catchups.RetryDelivery(req.Context(), id); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"message": "Catch Up delivery retried"})
		return
	}
	httpx.Error(w, http.StatusNotFound, "not found")
}

// writeCatchUpError 将业务校验错误转换成当前界面的语言和 HTTP 状态。
func (r *Router) writeCatchUpError(w http.ResponseWriter, req *http.Request, err error) {
	switch {
	case errors.Is(err, catchup.ErrInvalidDateRange):
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请选择有效的日期范围。", "Choose a valid date range."))
	case errors.Is(err, catchup.ErrDateRangeTooLong):
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "快速回顾最多支持 90 天。", "Catch Up supports at most 90 days."))
	case errors.Is(err, catchup.ErrDateRangeInFuture):
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "快速回顾的截止日期最晚为昨天。", "The latest Catch Up end date is yesterday."))
	case errors.Is(err, catchup.ErrBotUnavailable):
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "此功能要求先配置 Telegram Bot。", "Configure Telegram Bot before using this option."))
	case errors.Is(err, store.ErrInvalidCatchUpChats):
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "部分群组已不可用或未开启 AI 摘要，请重新选择。", "Some chats are unavailable or no longer have AI summaries enabled."))
	case errors.Is(err, store.ErrNoCatchUpSources):
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "所选范围内没有可用于快速回顾的摘要。", "No summaries are available for Catch Up in the selected range."))
	default:
		httpx.Error(w, http.StatusInternalServerError, err.Error())
	}
}

// positiveQueryInt 读取正整数分页参数，空值使用默认值。
func positiveQueryInt(req *http.Request, key string, fallback int) (int, error) {
	value := strings.TrimSpace(req.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}
