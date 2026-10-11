package api

import (
	"net/http"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// handleChatDefaults 读取或保存新群组默认配置。
func (r *Router) handleChatDefaults(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		defaults, err := r.store.Settings.GetChatDefaults(req.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, defaults)
	case http.MethodPut:
		var payload model.ChatDefaults
		if err := httpx.DecodeJSON(req, &payload); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		defaults, err := payload.Normalize()
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "默认配置无效：交付方式需为 dashboard 或 bot，摘要时间格式为 HH:MM。", "Invalid defaults: delivery mode must be dashboard or bot, and summary time must use HH:MM."))
			return
		}
		if err := r.store.Settings.SaveChatDefaults(req.Context(), defaults); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, defaults)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleApplyChatDefaults 把已保存的默认配置覆盖到全部已有群组。
func (r *Router) handleApplyChatDefaults(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defaults, err := r.store.Settings.GetChatDefaults(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, err := r.store.Chats.ApplyDefaults(req.Context(), defaults)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"updated": updated})
}
