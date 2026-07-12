package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
)

func (r *Router) handleTelegramAccounts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	accounts, err := r.store.Auth.List(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, accounts)
}

func (r *Router) handleTelegramAccountByID(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/api/telegram/accounts/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	accountID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || accountID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid telegram account id")
		return
	}
	if len(parts) == 2 && parts[1] == "sync" {
		r.handleSyncTelegramAccount(w, req, accountID)
		return
	}
	if len(parts) != 1 || req.Method != http.MethodDelete {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	count, err := r.store.AccountChats.CountCollectedBy(req.Context(), accountID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		httpx.Error(w, http.StatusConflict, r.localized(req.Context(), "请先为正在使用该账号的群组选择其他账号。", "Choose another account for the chats currently using this account first."))
		return
	}
	r.telegram.StopAccount(accountID)
	r.telegram.CancelAuth(accountID)
	if err := r.store.Auth.Delete(req.Context(), accountID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleSyncTelegramAccount(w http.ResponseWriter, req *http.Request, accountID int64) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.telegram.SyncChats(req.Context(), accountID); err != nil {
		if floodErr, ok := asFloodWaitError(err); ok {
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, floodErr.Error(), "telegram_flood_wait", floodErr.RetryAfterSeconds())
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
