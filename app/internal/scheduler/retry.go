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

// deliveryRetryDue 判断自动投递是否可以进行：从未失败过可直接发送；失败后需仍在当前重试上限内，且到达下一次计划时间。
func deliveryRetryDue(settings model.AppSettings, item model.Summary, now time.Time) bool {
	if item.DeliveryRetryCount == 0 {
		return true
	}
	// 计数为已失败次数，只有失败前计数小于上限才会安排重试，因此计数不超过当前上限才放行。
	if settings.SummaryRetryLimit <= 0 || item.DeliveryRetryCount > settings.SummaryRetryLimit {
		return false
	}
	if item.NextDeliveryRetryAt == nil {
		return false
	}
	return !item.NextDeliveryRetryAt.After(now)
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
