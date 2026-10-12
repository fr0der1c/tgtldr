package scheduler

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/dailydigest"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

// TestSummaryFrequency 验证每周和每月摘要的生成时机、覆盖范围、单独推送、手动生成整周期以及停机后只补最近一期。
func TestSummaryFrequency(t *testing.T) {
	st := schedulerTestStore(t)
	ctx := t.Context()
	settings, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.DefaultTimezone = "Asia/Shanghai"
	settings.Language = model.LanguageZhCN
	settings.OpenAIBaseURL = "https://model.test/v1"
	settings.OpenAIAPIKey = "test"
	settings.OpenAIModel = "test-model"
	settings.OpenAIRequestMode = model.OpenAIRequestModeNonStream
	settings.BotEnabled = true
	settings.BotToken = "test-token"
	settings.BotTargetChatID = "123"
	settings.BotSummaryDeliveryMode = model.BotSummaryDeliveryModeDailyDigest
	if _, err = st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	_, err = st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode,summary_time_local,summary_frequency) overriding system value values
 (1,1,'weekly',true,'bot','09:00','week'),(2,2,'daily',true,'bot','09:00','day'),(3,3,'monthly',true,'dashboard','09:00','month');
 insert into messages(chat_id,telegram_message_id,text_content,message_time) values
 (1,1,'weekly-monday','2026-09-28T10:00:00+08:00'),(1,2,'weekly-wednesday','2026-09-30T10:00:00+08:00'),
 (1,3,'weekly-sunday','2026-10-04T23:00:00+08:00'),(1,4,'next-week','2026-10-05T08:00:00+08:00'),
 (1,5,'later-week','2026-10-13T10:00:00+08:00'),
 (2,1,'daily-message','2026-10-04T10:00:00+08:00'),
 (3,1,'monthly-first','2026-09-01T10:00:00+08:00'),(3,2,'monthly-last','2026-09-30T22:00:00+08:00'),(3,3,'october','2026-10-01T10:00:00+08:00')`)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var prompts, sent []string
	oldTransport := http.DefaultTransport
	http.DefaultTransport = recoveryTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Host == "api.telegram.org" {
			sent = append(sent, string(body))
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)), Header: make(http.Header)}, nil
		}
		prompts = append(prompts, string(body))
		result := `{"model":"test-model","choices":[{"message":{"role":"assistant","content":"generated-content"}}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(result)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	clk := &testClock{now: time.Date(2026, 10, 5, 8, 59, 0, 0, shanghai)}
	botService := bot.New()
	digests := dailydigest.NewService(ctx, st, clk, botService, time.Second)
	service := NewService(st, clk, summary.NewService(st, clk, time.Second), botService, digests)
	promptFor := func(text string) string {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		for _, prompt := range prompts {
			if strings.Contains(prompt, text) {
				return prompt
			}
		}
		t.Fatalf("no model request contains %q", text)
		return ""
	}
	countSummaries := func(chatID int64) int {
		t.Helper()
		var count int
		if err := st.Pool.QueryRow(ctx, `select count(*) from summaries where chat_id=$1`, chatID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}

	// 摘要时间之前不生成。
	if err = service.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if countSummaries(1)+countSummaries(3) != 0 {
		t.Fatal("generated before the summary time")
	}

	// 周一摘要时间：周报覆盖上周一到周日，单独推送并显示完整范围；月报覆盖上个整月。
	clk.Set(time.Date(2026, 10, 5, 9, 0, 0, 0, shanghai))
	if err = service.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	weekly, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-28")
	if err != nil || weekly.Period != model.SummaryPeriodWeek || weekly.PeriodEnd != "2026-10-04" || weekly.SourceMessageCount != 3 ||
		weekly.BotSummaryDeliveryMode != model.BotSummaryDeliveryModePerChat || weekly.DeliveredAt == nil {
		t.Fatalf("weekly summary: %+v %v", weekly, err)
	}
	prompt := promptFor("weekly-monday")
	if !strings.Contains(prompt, "2026-09-30 10:00") || !strings.Contains(prompt, "2026-10-04") || strings.Contains(prompt, "next-week") {
		t.Fatalf("weekly prompt: %s", prompt)
	}
	mu.Lock()
	delivered := strings.Join(sent, "\n")
	mu.Unlock()
	if !strings.Contains(delivered, "2026-09-28 至 2026-10-04") {
		t.Fatalf("weekly delivery header: %s", delivered)
	}
	monthly, err := st.Summaries.GetByChatAndDate(ctx, 3, "2026-09-01")
	if err != nil || monthly.Period != model.SummaryPeriodMonth || monthly.PeriodEnd != "2026-09-30" || monthly.SourceMessageCount != 2 {
		t.Fatalf("monthly summary: %+v %v", monthly, err)
	}
	dailySummary, err := st.Summaries.GetByChatAndDate(ctx, 2, "2026-10-04")
	if err != nil || dailySummary.Period != model.SummaryPeriodDay || dailySummary.BotSummaryDeliveryMode != model.BotSummaryDeliveryModeDailyDigest {
		t.Fatalf("daily summary: %+v %v", dailySummary, err)
	}
	var digestChats []int64
	if err = st.Pool.QueryRow(ctx, `select coalesce(array_agg(chat_id order by chat_id), '{}') from daily_digest_sources`).Scan(&digestChats); err != nil ||
		len(digestChats) != 1 || digestChats[0] != 2 {
		t.Fatalf("daily digest sources: %v %v", digestChats, err)
	}

	// 按日期筛选和快速回顾会找到覆盖这一天的周报，并保留周期范围。
	found, err := st.Summaries.Search(ctx, store.SummaryListParams{ChatID: 1, DateFrom: "2026-09-30", DateTo: "2026-09-30"})
	if err != nil || len(found.Items) != 1 || found.Items[0].SummaryDate != "2026-09-28" {
		t.Fatalf("search by a day inside the week: %+v %v", found.Items, err)
	}
	catchUp, err := st.CatchUps.Create(ctx, "2026-09-30", "2026-10-02", []int64{1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if catchUp, err = st.CatchUps.GetForGeneration(ctx, catchUp.ID); err != nil || len(catchUp.Sources) != 1 || catchUp.Sources[0].PeriodEnd != "2026-10-04" {
		t.Fatalf("catch-up sources: %+v %v", catchUp.Sources, err)
	}

	// 周二再跑不会产生新的周报；手动选择周中某天会重新生成它所在的整周。
	clk.Set(time.Date(2026, 10, 6, 9, 30, 0, 0, shanghai))
	if err = service.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if countSummaries(1) != 1 || countSummaries(3) != 1 {
		t.Fatal("generated again within the same period")
	}
	chat, err := st.Chats.GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RunNow(ctx, chat, "2026-10-01"); err != nil {
		t.Fatal(err)
	}
	if regenerated, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-28"); err != nil || countSummaries(1) != 1 || !regenerated.GeneratedAt.After(weekly.GeneratedAt) {
		t.Fatalf("manual run did not regenerate the week: %+v %v", regenerated, err)
	}
	if err = service.RunNow(ctx, chat, "2026-10-14"); err != nil {
		t.Fatal(err)
	}
	current, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-10-12")
	if err != nil || current.Period != model.SummaryPeriodWeek || current.SourceMessageCount != 1 {
		t.Fatalf("manual run for another week: %+v %v", current, err)
	}
	// 手动发送也不能推送周期尚未结束的部分内容。
	if err = service.RetryDelivery(ctx, current.ID); err == nil || !strings.Contains(err.Error(), "周期尚未结束") {
		t.Fatalf("partial week manually delivered: %v", err)
	}

	// 周期结束前手动生成的周报只是部分内容，周期结束后的摘要时间会重新生成。
	clk.Set(time.Date(2026, 10, 19, 9, 0, 0, 0, shanghai))
	if err = service.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if regenerated, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-10-12"); err != nil ||
		regenerated.GeneratedAt.Before(time.Date(2026, 10, 19, 0, 0, 0, 0, shanghai)) || regenerated.DeliveredAt == nil {
		t.Fatalf("partial week not regenerated: %+v %v", regenerated, err)
	}
	// 部分周报不推送，周期结束后的完整周报只推送一次。
	mu.Lock()
	partialDeliveries := 0
	for _, body := range sent {
		if strings.Contains(body, "2026-10-12") {
			partialDeliveries++
		}
	}
	mu.Unlock()
	if partialDeliveries != 1 {
		t.Fatalf("week of 2026-10-12 delivered %d times", partialDeliveries)
	}
	if _, err = st.Pool.Exec(ctx, `delete from summaries where chat_id=1 and summary_date='2026-10-12'`); err != nil {
		t.Fatal(err)
	}

	// 昨天留有其他周期的摘要（例如切换频率前手动生成的周报）时，定时任务不处理它，避免反复重新生成。
	// 这一行也不能让当天的每日总览一直等待：另一个日报群照常进入总览。
	if _, err = st.Pool.Exec(ctx, `insert into summaries(chat_id,summary_date,status,content,period,generated_at) values (2,'2026-10-27','succeeded','stale-week','week',now());
 insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode,summary_time_local) overriding system value values (4,4,'daily-other',true,'bot','09:00');
 insert into messages(chat_id,telegram_message_id,text_content,message_time) values (4,1,'other-daily','2026-10-27T10:00:00+08:00')`); err != nil {
		t.Fatal(err)
	}
	stale, err := st.Summaries.GetByChatAndDate(ctx, 2, "2026-10-27")
	if err != nil {
		t.Fatal(err)
	}

	// 停机多周后只补最近一个已结束的周期，不追溯更早的周。
	clk.Set(time.Date(2026, 10, 28, 9, 0, 0, 0, shanghai))
	if err = service.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Summaries.GetByChatAndDate(ctx, 1, "2026-10-19"); err != nil {
		t.Fatalf("latest ended week not generated: %v", err)
	}
	if countSummaries(1) != 2 {
		t.Fatal("older missed weeks were generated")
	}
	if kept, err := st.Summaries.GetByChatAndDate(ctx, 2, "2026-10-27"); err != nil || kept.Content != "stale-week" || !kept.GeneratedAt.Equal(stale.GeneratedAt) {
		t.Fatalf("stale period summary was regenerated: %+v %v", kept, err)
	}
	if err = st.Pool.QueryRow(ctx, `select coalesce(array_agg(s.chat_id order by s.chat_id), '{}') from daily_digest_sources s
 join daily_digests d on d.id = s.daily_digest_id where d.summary_date = '2026-10-27'`).Scan(&digestChats); err != nil ||
		len(digestChats) != 1 || digestChats[0] != 4 {
		t.Fatalf("daily digest blocked by stale period summary: %v %v", digestChats, err)
	}
	if _, err = st.Pool.Exec(ctx, `update chats set summary_enabled = false where id = 4`); err != nil {
		t.Fatal(err)
	}

	// 日报群在周中失败时，失败通知不等待当天没有摘要的每周和每月群组。
	if _, err = st.Pool.Exec(ctx, `insert into summaries(chat_id,summary_date,status,error_message) values (2,'2026-10-07','failed','model error')`); err != nil {
		t.Fatal(err)
	}
	states, err := st.FailureStates(ctx, "2026-10-07")
	if err != nil || len(states) != 1 || states[0].Status != "failed" || !states[0].InDigest {
		t.Fatalf("failure states: %+v %v", states, err)
	}
	// 周一既有周报（单独推送）又有缺失的日报（仍在等待）；没有这天摘要的月报群组不参与。
	if states, err = st.FailureStates(ctx, "2026-09-28"); err != nil || len(states) != 2 {
		t.Fatalf("weekly failure states: %+v %v", states, err)
	}
	for _, state := range states {
		if state.Status == "succeeded" == state.InDigest {
			t.Fatalf("weekly failure states: %+v", states)
		}
	}

	// 切换频率按默认时区中的今天计算生效日期，保存后可以读回。
	daily, err := st.Chats.GetByID(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ChangeSummaryFrequency(ctx, &daily, model.SummaryPeriodWeek); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Chats.Save(ctx, daily); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Chats.GetByID(ctx, 2)
	if err != nil || saved.SummaryFrequency != model.SummaryPeriodWeek || saved.PreviousSummaryFrequency != model.SummaryPeriodDay ||
		saved.SummaryFrequencyEffectiveDate != "2026-11-02" {
		t.Fatalf("saved frequency: %+v %v", saved, err)
	}
}
