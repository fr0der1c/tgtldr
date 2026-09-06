package scheduler

import (
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

func shouldRetrySummary(settings model.AppSettings, item model.Summary, now time.Time) bool {
	if settings.SummaryRetryLimit <= 0 {
		return false
	}
	if item.RetryCount >= settings.SummaryRetryLimit {
		return false
	}
	if item.NextRetryAt == nil {
		return false
	}
	return !item.NextRetryAt.After(now)
}

func nextSummaryRetryAt(settings model.AppSettings, retryCount int, now time.Time) (time.Time, bool) {
	if settings.SummaryRetryLimit <= 0 {
		return time.Time{}, false
	}
	if retryCount >= settings.SummaryRetryLimit {
		return time.Time{}, false
	}
	return now.Add(model.SummaryRetryDelay(retryCount + 1)), true
}
