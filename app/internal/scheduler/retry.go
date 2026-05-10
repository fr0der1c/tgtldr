package scheduler

import (
	"math"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
)

const maxRetryBackoffDuration = time.Duration(math.MaxInt64)

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
	return now.Add(summaryRetryBackoffDelay(settings, retryCount+1)), true
}

func summaryRetryBackoffDelay(settings model.AppSettings, retryNumber int) time.Duration {
	baseMinutes := settings.SummaryRetryBackoffBaseMinutes
	if baseMinutes < 1 {
		baseMinutes = model.DefaultSummaryRetryBackoffBaseMinutes
	}
	multiplier := settings.SummaryRetryBackoffMultiplier
	if multiplier < 1 {
		multiplier = model.DefaultSummaryRetryBackoffMultiplier
	}
	if retryNumber < 1 {
		retryNumber = 1
	}

	minutes := float64(baseMinutes) * math.Pow(multiplier, float64(retryNumber-1))
	maxMinutes := float64(maxRetryBackoffDuration / time.Minute)
	if math.IsInf(minutes, 0) || minutes > maxMinutes {
		return maxRetryBackoffDuration
	}
	return time.Duration(minutes * float64(time.Minute))
}
