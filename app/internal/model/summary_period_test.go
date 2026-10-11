package model

import "testing"

func TestSummaryPeriodBounds(t *testing.T) {
	for _, tc := range []struct {
		period     SummaryPeriod
		date       string
		start, end string
		days       int
	}{
		{SummaryPeriodDay, "2026-10-08", "2026-10-08", "2026-10-08", 1},
		{SummaryPeriodWeek, "2026-10-08", "2026-10-05", "2026-10-11", 7},
		{SummaryPeriodWeek, "2026-10-11", "2026-10-05", "2026-10-11", 7},
		{SummaryPeriodWeek, "2026-10-05", "2026-10-05", "2026-10-11", 7},
		{SummaryPeriodMonth, "2026-10-31", "2026-10-01", "2026-10-31", 31},
		{SummaryPeriodMonth, "2028-02-10", "2028-02-01", "2028-02-29", 29},
		{"unknown", "2026-10-08", "2026-10-08", "2026-10-08", 1},
	} {
		start, err := PeriodStartDate(tc.period, tc.date)
		if err != nil || start != tc.start {
			t.Fatalf("%s %s start=%s %v", tc.period, tc.date, start, err)
		}
		end, err := PeriodEndDate(tc.period, start)
		days, _ := PeriodDays(tc.period, start)
		if err != nil || end != tc.end || days != tc.days {
			t.Fatalf("%s %s end=%s days=%d %v", tc.period, tc.date, end, days, err)
		}
	}
	item := Summary{SummaryDate: "2026-10-05", Period: SummaryPeriodWeek}
	item.FillPeriodEnd()
	if item.PeriodEnd != "2026-10-11" {
		t.Fatalf("period end %+v", item)
	}
	item = Summary{SummaryDate: "2026-10-05"}
	item.FillPeriodEnd()
	if item.Period != SummaryPeriodDay || item.PeriodEnd != "2026-10-05" {
		t.Fatalf("legacy summary %+v", item)
	}
}

// TestChangeSummaryFrequency 验证切换后当前一期按原频率完成，新频率从下一个完整周期开始，中间不重叠也不留空。
func TestChangeSummaryFrequency(t *testing.T) {
	for _, tc := range []struct {
		name      string
		from, to  SummaryPeriod
		today     string
		effective string
		periods   map[string]SummaryPeriod
	}{
		{"daily to weekly", SummaryPeriodDay, SummaryPeriodWeek, "2026-10-07", "2026-10-12",
			map[string]SummaryPeriod{"2026-10-07": SummaryPeriodDay, "2026-10-11": SummaryPeriodDay, "2026-10-12": SummaryPeriodWeek}},
		{"weekly to daily", SummaryPeriodWeek, SummaryPeriodDay, "2026-10-07", "2026-10-12",
			map[string]SummaryPeriod{"2026-10-05": SummaryPeriodWeek, "2026-10-11": SummaryPeriodWeek, "2026-10-12": SummaryPeriodDay}},
		{"weekly to monthly", SummaryPeriodWeek, SummaryPeriodMonth, "2026-10-14", "2026-11-01",
			map[string]SummaryPeriod{"2026-10-18": SummaryPeriodWeek, "2026-10-25": SummaryPeriodWeek, "2026-10-26": SummaryPeriodDay, "2026-10-31": SummaryPeriodDay, "2026-11-01": SummaryPeriodMonth}},
		{"monthly to weekly", SummaryPeriodMonth, SummaryPeriodWeek, "2026-10-14", "2026-11-02",
			map[string]SummaryPeriod{"2026-10-31": SummaryPeriodMonth, "2026-11-01": SummaryPeriodDay, "2026-11-02": SummaryPeriodWeek}},
		{"daily to monthly on the first", SummaryPeriodDay, SummaryPeriodMonth, "2026-10-01", "2026-11-01",
			map[string]SummaryPeriod{"2026-10-01": SummaryPeriodDay, "2026-10-31": SummaryPeriodDay, "2026-11-01": SummaryPeriodMonth}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat := Chat{SummaryFrequency: tc.from}
			if err := chat.ChangeSummaryFrequency(tc.to, tc.today); err != nil {
				t.Fatal(err)
			}
			if chat.SummaryFrequency != tc.to || chat.PreviousSummaryFrequency != tc.from || chat.SummaryFrequencyEffectiveDate != tc.effective {
				t.Fatalf("change: %+v", chat)
			}
			for date, want := range tc.periods {
				if got := chat.SummaryPeriodFor(date); got != want {
					t.Fatalf("%s: got %s want %s", date, got, want)
				}
			}
			saved := [3]string{string(chat.SummaryFrequency), string(chat.PreviousSummaryFrequency), chat.SummaryFrequencyEffectiveDate}
			if err := chat.ChangeSummaryFrequency(tc.to, "2026-10-09"); err != nil ||
				saved != [3]string{string(chat.SummaryFrequency), string(chat.PreviousSummaryFrequency), chat.SummaryFrequencyEffectiveDate} {
				t.Fatalf("saving the same frequency changed the pending switch: %+v", chat)
			}
		})
	}
	var chat Chat
	if err := chat.ChangeSummaryFrequency(SummaryPeriodDay, "2026-10-07"); err != nil || chat.SummaryFrequencyEffectiveDate != "" {
		t.Fatalf("default daily: %+v %v", chat, err)
	}
	if chat.SummaryPeriodFor("2026-10-07") != SummaryPeriodDay {
		t.Fatal("legacy chat is not daily")
	}
}

func TestLatestEndedPeriod(t *testing.T) {
	weekly := Chat{SummaryFrequency: SummaryPeriodWeek}
	monthly := Chat{SummaryFrequency: SummaryPeriodMonth}
	switched := Chat{SummaryFrequency: SummaryPeriodWeek, PreviousSummaryFrequency: SummaryPeriodMonth, SummaryFrequencyEffectiveDate: "2026-11-02"}
	for _, tc := range []struct {
		chat    Chat
		lastDay string
		start   string
		period  SummaryPeriod
	}{
		{Chat{}, "2026-10-07", "2026-10-07", SummaryPeriodDay},
		{weekly, "2026-10-11", "2026-10-05", SummaryPeriodWeek},
		{weekly, "2026-10-13", "2026-10-05", SummaryPeriodWeek},
		{monthly, "2026-10-31", "2026-10-01", SummaryPeriodMonth},
		{monthly, "2026-10-12", "2026-09-01", SummaryPeriodMonth},
		{switched, "2026-11-01", "2026-11-01", SummaryPeriodDay},
		{switched, "2026-11-05", "2026-11-01", SummaryPeriodDay},
		{switched, "2026-11-08", "2026-11-02", SummaryPeriodWeek},
	} {
		start, period, err := tc.chat.LatestEndedPeriod(tc.lastDay)
		if err != nil || start != tc.start || period != tc.period {
			t.Fatalf("%+v %s: %s %s %v", tc.chat, tc.lastDay, start, period, err)
		}
	}
}
