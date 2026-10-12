package scheduler

import (
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

func TestSummaryPeriodDelivery(t *testing.T) {
	weekly := model.Summary{SummaryDate: "2026-09-28", Period: model.SummaryPeriodWeek}
	weekly.FillPeriodEnd()
	daily := model.Summary{SummaryDate: "2026-10-04"}
	daily.FillPeriodEnd()
	if got := summaryDateLabel(weekly, model.LanguageZhCN); got != "2026-09-28 至 2026-10-04" {
		t.Fatalf("zh label %q", got)
	}
	if got := summaryDateLabel(weekly, model.LanguageEN); got != "2026-09-28 to 2026-10-04" {
		t.Fatalf("en label %q", got)
	}
	if got := summaryDateLabel(daily, model.LanguageEN); got != "2026-10-04" {
		t.Fatalf("daily label %q", got)
	}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	// 周期结束前生成的周报在周期结束后需要重新生成，不能直接推送。
	weekly.GeneratedAt = time.Date(2026, 10, 1, 12, 0, 0, 0, shanghai)
	if summaryReadyForDelivery(weekly, "Asia/Shanghai") {
		t.Fatal("partial week considered ready")
	}
	weekly.GeneratedAt = time.Date(2026, 10, 5, 0, 0, 0, 0, shanghai)
	if !summaryReadyForDelivery(weekly, "Asia/Shanghai") {
		t.Fatal("finished week not ready")
	}
	settings := model.AppSettings{BotSummaryDeliveryMode: model.BotSummaryDeliveryModeDailyDigest}
	if resolveDeliveryMode(settings, "2026-09-28", model.SummaryPeriodWeek) != model.BotSummaryDeliveryModePerChat ||
		resolveDeliveryMode(settings, "2026-09-28", model.SummaryPeriodDay) != model.BotSummaryDeliveryModeDailyDigest {
		t.Fatal("weekly summaries must be delivered on their own")
	}
}

func TestFailureMessagePeriod(t *testing.T) {
	states := []store.FailureState{{Kind: "summary", Status: "failed", Error: "model not found", PeriodEnd: "2026-10-04"}}
	message, ready := failureMessage("2026-09-28", states, 4, model.LanguageZhCN, "", false)
	if !ready || !strings.Contains(message, "2026-09-28 至 2026-10-04") {
		t.Fatalf("weekly failure notice: %q", message)
	}
	states[0].PeriodEnd = "2026-09-28"
	if message, _ = failureMessage("2026-09-28", states, 4, model.LanguageEN, "", false); strings.Contains(message, " to ") {
		t.Fatalf("daily failure notice: %q", message)
	}
}
