package api

import (
	"net/http"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
)

// handleSummaryRecovery 创建全量失败任务批次，或查询后台队列的逐项进度。
func (r *Router) handleSummaryRecovery(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if req.Method == http.MethodPost {
		if err := r.store.QueueFailedSummaries(req.Context()); err != nil {
			httpx.Error(w, 500, err.Error())
			return
		}
	}
	items, err := r.store.RecoveryItems(req.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// handleFailureNotices 显示通知的发送结果与重试次数。
func (r *Router) handleFailureNotices(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, 405, "method not allowed")
		return
	}
	items, err := r.store.FailureNotices(req.Context())
	if err != nil {
		httpx.Error(w, 500, err.Error())
		return
	}
	httpx.JSON(w, 200, items)
}
