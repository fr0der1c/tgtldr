package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/fr0der1c/tgtldr/app/internal/catchup"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/scheduler"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

// handleSummaryBatch 提交批量生成或查询当前批次的逐项进度。
func (r *Router) handleSummaryBatch(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.writeSummaryBatch(w, req, scheduler.SummaryBatchPlan{})
	case http.MethodPost:
		var payload struct {
			ChatIDs   []int64 `json:"chatIds"`
			FromDate  string  `json:"fromDate"`
			ToDate    string  `json:"toDate"`
			Overwrite bool    `json:"overwrite"`
		}
		if err := httpx.DecodeJSON(req, &payload); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		plan, err := r.scheduler.QueueSummaryBatch(req.Context(), payload.ChatIDs, payload.FromDate, payload.ToDate, payload.Overwrite)
		if err != nil {
			r.writeSummaryBatchError(w, req, err)
			return
		}
		r.writeSummaryBatch(w, req, plan)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (r *Router) writeSummaryBatch(w http.ResponseWriter, req *http.Request, plan scheduler.SummaryBatchPlan) {
	items, err := r.store.SummaryBatchItems(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"queued": plan.Queued, "skipped": plan.Skipped, "items": items})
}

// writeSummaryBatchError 将批量生成的校验错误转换成当前界面的语言和 HTTP 状态。
func (r *Router) writeSummaryBatchError(w http.ResponseWriter, req *http.Request, err error) {
	ctx := req.Context()
	switch {
	case errors.Is(err, catchup.ErrInvalidDateRange):
		httpx.Error(w, http.StatusBadRequest, r.localized(ctx, "请选择有效的日期范围。", "Choose a valid date range."))
	case errors.Is(err, catchup.ErrDateRangeTooLong):
		httpx.Error(w, http.StatusBadRequest, r.localized(ctx,
			fmt.Sprintf("批量生成最多支持 %d 天。", catchup.MaximumDateRangeDays),
			fmt.Sprintf("Batch generation supports at most %d days.", catchup.MaximumDateRangeDays)))
	case errors.Is(err, catchup.ErrDateRangeInFuture):
		httpx.Error(w, http.StatusBadRequest, r.localized(ctx, "批量生成的截止日期最晚为昨天。", "The latest batch end date is yesterday."))
	case errors.Is(err, scheduler.ErrSummaryBatchChats):
		httpx.Error(w, http.StatusBadRequest, r.localized(ctx, "部分群组已不可用或未开启 AI 总结，请重新选择。", "Some chats are unavailable or no longer have AI summaries enabled."))
	case errors.Is(err, scheduler.ErrSummaryBatchTooLarge):
		httpx.Error(w, http.StatusBadRequest, r.localized(ctx,
			fmt.Sprintf("单批最多生成 %d 条摘要，请减少群组或缩短日期范围。", scheduler.SummaryBatchMaxItems),
			fmt.Sprintf("A batch can generate at most %d summaries. Select fewer chats or a shorter range.", scheduler.SummaryBatchMaxItems)))
	case errors.Is(err, store.ErrSummaryBatchActive):
		httpx.Error(w, http.StatusConflict, r.localized(ctx, "上一批批量生成尚未完成，请稍后再提交。", "The previous batch is still running. Try again after it finishes."))
	default:
		httpx.Error(w, http.StatusInternalServerError, err.Error())
	}
}
