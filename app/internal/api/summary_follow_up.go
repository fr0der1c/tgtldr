package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
	"github.com/jackc/pgx/v5"
)

func (r *Router) handleSummaryFollowUp(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(req.PathValue("summaryID"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid summary id")
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, 1024*1024)
	var payload struct {
		Question string                 `json:"question"`
		History  []summary.FollowUpTurn `json:"history"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid follow-up request")
		return
	}
	question, history, err := summary.NormalizeFollowUp(payload.Question, payload.History)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := r.store.Summaries.GetByID(req.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "summary not found")
		} else {
			httpx.Error(w, http.StatusInternalServerError, "unable to load summary")
		}
		return
	}
	answer, err := r.scheduler.AskFollowUp(req.Context(), item, question, history)
	if err != nil {
		if errors.Is(err, summary.ErrFollowUpContextLimit) {
			httpx.ErrorWithCode(w, http.StatusBadRequest, err.Error(), "follow_up_context_limit", 0)
		} else if errors.Is(err, summary.ErrInvalidFollowUp) {
			httpx.Error(w, http.StatusBadRequest, err.Error())
		} else {
			httpx.Error(w, http.StatusInternalServerError, "unable to answer follow-up")
		}
		return
	}
	httpx.JSON(w, http.StatusOK, answer)
}
