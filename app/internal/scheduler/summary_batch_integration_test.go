package scheduler

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/catchup"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/dailydigest"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
)

// TestSummaryBatch 验证批量生成的校验、跳过成功摘要、并发上限、逐项结果、批次互斥和中断恢复。
func TestSummaryBatch(t *testing.T) {
	st := schedulerTestStore(t)
	ctx := t.Context()
	settings, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.DefaultTimezone = "Asia/Shanghai"
	settings.OpenAIBaseURL = "https://model.test/v1"
	settings.OpenAIAPIKey = "test"
	settings.OpenAIModel = "test-model"
	settings.OpenAIRequestMode = model.OpenAIRequestModeNonStream
	if _, err = st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	_, err = st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode) overriding system value values
 (1,1,'first',true,'dashboard'),(2,2,'second',true,'dashboard'),(3,3,'disabled',false,'dashboard'),(4,4,'third',true,'dashboard');
 insert into summaries(chat_id,summary_date,status,content,source_message_count) values (1,'2026-09-01','succeeded','kept-content',1);
 insert into messages(chat_id,telegram_message_id,text_content,message_time) values
 (1,1,'normal message','2026-09-01T10:00:00+08:00'),(1,2,'normal message','2026-09-02T10:00:00+08:00'),
 (2,1,'normal message','2026-09-01T10:00:00+08:00'),(2,2,'permanent-error','2026-09-02T10:00:00+08:00'),
 (4,1,'temporary-error','2026-09-05T10:00:00+08:00')`)
	if err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = recoveryTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		code, result := 200, `{"model":"test-model","choices":[{"message":{"role":"assistant","content":"batch-content"}}]}`
		if strings.Contains(string(body), "permanent-error") {
			code, result = 404, `{"error":{"message":"model not found"}}`
		}
		if strings.Contains(string(body), "temporary-error") {
			code, result = 503, `{"error":{"message":"overloaded"}}`
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(result)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	botService := bot.New()
	digests := dailydigest.NewService(ctx, st, clock.System{}, botService, time.Second)
	service := NewService(st, clock.System{}, summary.NewService(st, clock.System{}, time.Second), botService, digests)

	if _, err = service.QueueSummaryBatch(ctx, []int64{1, 3}, "2026-09-01", "2026-09-02", false); !errors.Is(err, ErrSummaryBatchChats) {
		t.Fatalf("disabled chat: %v", err)
	}
	if _, err = service.QueueSummaryBatch(ctx, []int64{1, 2, 4}, "2026-06-01", "2026-08-09", false); !errors.Is(err, ErrSummaryBatchTooLarge) {
		t.Fatalf("too large: %v", err)
	}
	future := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	if _, err = service.QueueSummaryBatch(ctx, []int64{1}, "2026-09-01", future, false); !errors.Is(err, catchup.ErrDateRangeInFuture) {
		t.Fatalf("future: %v", err)
	}

	plan, err := service.QueueSummaryBatch(ctx, []int64{1, 2, 1}, "2026-09-01", "2026-09-02", false)
	if err != nil || plan.Queued != 3 || plan.Skipped != 1 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	if _, err = service.QueueSummaryBatch(ctx, []int64{4}, "2026-09-01", "2026-09-01", false); !errors.Is(err, store.ErrSummaryBatchActive) {
		t.Fatalf("active batch: %v", err)
	}
	if _, err = service.QueueSummaryBatch(ctx, []int64{1}, "2026-09-01", "2026-09-01", false); !errors.Is(err, store.ErrSummaryBatchActive) {
		t.Fatalf("active batch with nothing to queue: %v", err)
	}

	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := st.SummaryBatchItems(ctx)
	if err != nil || len(items) != 3 || items[0].Status == "queued" || items[1].Status == "queued" || items[2].Status != "queued" {
		t.Fatalf("first round: %+v %v", items, err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = st.SummaryBatchItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, item := range items {
		got[item.ChatTitle+" "+item.Date] = item.Status
	}
	want := map[string]string{"second 2026-09-01": "succeeded", "first 2026-09-02": "succeeded", "second 2026-09-02": "failed"}
	if len(got) != len(want) {
		t.Fatalf("items: %+v", items)
	}
	for key, status := range want {
		if got[key] != status {
			t.Fatalf("items: %+v", items)
		}
	}
	kept, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-01")
	if err != nil || kept.Content != "kept-content" {
		t.Fatalf("skipped summary changed: %+v %v", kept, err)
	}

	// 入队后才变化的条件在执行前复查：已生成成功的跳过，关闭 AI 总结的群组跳过，跳过项不占用本轮名额。
	plan, err = service.QueueSummaryBatch(ctx, []int64{2, 4, 1}, "2026-09-03", "2026-09-03", false)
	if err != nil || plan.Queued != 3 {
		t.Fatalf("recheck plan: %+v %v", plan, err)
	}
	if _, err = st.Pool.Exec(ctx, `insert into summaries(chat_id,summary_date,status,content,source_message_count) values (2,'2026-09-03','succeeded','done-elsewhere',1);
 update chats set summary_enabled=false where id=4`); err != nil {
		t.Fatal(err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = st.SummaryBatchItems(ctx)
	if err != nil || len(items) != 3 || items[0].Status != "skipped" || items[1].Status != "skipped" || items[2].Status == "queued" {
		t.Fatalf("recheck: %+v %v", items, err)
	}
	if kept, err = st.Summaries.GetByChatAndDate(ctx, 2, "2026-09-03"); err != nil || kept.Content != "done-elsewhere" {
		t.Fatalf("summary generated elsewhere was overwritten: %+v %v", kept, err)
	}

	// 可重试的失败保持排队，等到不再自动重试时才记为失败。
	if _, err = st.Pool.Exec(ctx, `update chats set summary_enabled=true where id=4`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QueueSummaryBatch(ctx, []int64{4}, "2026-09-05", "2026-09-05", false); err != nil {
		t.Fatal(err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = st.SummaryBatchItems(ctx)
	if err != nil || len(items) != 1 || items[0].Status != "queued" || !items[0].Started {
		t.Fatalf("retrying item: %+v %v", items, err)
	}
	// 等待重试期间关闭 AI 总结后不会再自动重试，该项记为跳过而不是一直排队。
	if _, err = st.Pool.Exec(ctx, `update chats set summary_enabled=false where id=4`); err != nil {
		t.Fatal(err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = st.SummaryBatchItems(ctx)
	if err != nil || items[0].Status != "skipped" {
		t.Fatalf("disabled during retry: %+v %v", items, err)
	}
	if _, err = st.Pool.Exec(ctx, `update chats set summary_enabled=true where id=4`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QueueSummaryBatch(ctx, []int64{4}, "2026-09-05", "2026-09-05", false); err != nil {
		t.Fatal(err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Pool.Exec(ctx, `update summaries set next_retry_at=null where chat_id=4 and summary_date='2026-09-05'`); err != nil {
		t.Fatal(err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = st.SummaryBatchItems(ctx)
	if err != nil || items[0].Status != "failed" || items[0].Error == "" {
		t.Fatalf("retries exhausted: %+v %v", items, err)
	}

	plan, err = service.QueueSummaryBatch(ctx, []int64{1}, "2026-09-01", "2026-09-01", true)
	if err != nil || plan.Queued != 1 || plan.Skipped != 0 {
		t.Fatalf("overwrite plan: %+v %v", plan, err)
	}
	if err = service.runSummaryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	rerun, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-01")
	if err != nil || rerun.Content != "batch-content" {
		t.Fatalf("overwrite: %+v %v", rerun, err)
	}

	// 同一群组日期正在其他流程生成时原样留到下一轮；已开始却没有摘要记录（生成前进程中断）时退回未开始。
	if _, err = service.QueueSummaryBatch(ctx, []int64{1}, "2026-09-04", "2026-09-04", false); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Pool.Exec(ctx, `update summary_batch_items set started_at=now()`); err != nil {
		t.Fatal(err)
	}
	key := summaryTaskKey(1, "2026-09-04")
	if !service.beginTask(key) {
		t.Fatal("task already running")
	}
	runBatchRound := func(name string, check func(store.SummaryBatchItem) bool) {
		t.Helper()
		if err := service.runSummaryBatch(ctx); err != nil {
			t.Fatal(err)
		}
		items, err := st.SummaryBatchItems(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if !check(item) {
				t.Fatalf("%s: %+v", name, items)
			}
		}
	}
	runBatchRound("in-flight item", func(item store.SummaryBatchItem) bool { return item.Status == "queued" && item.Started })
	service.finishTask(key)
	runBatchRound("interrupted item", func(item store.SummaryBatchItem) bool { return item.Status == "queued" && !item.Started })
	runBatchRound("rerun item", func(item store.SummaryBatchItem) bool { return item.Status == "succeeded" })
	if _, err = st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-04"); err != nil {
		t.Fatalf("summary not generated: %v", err)
	}

	// 重新生成时，开始后未被本批更新的成功摘要和停在生成中的摘要都说明进程曾中断，退回未开始后重新生成。
	if _, err = service.QueueSummaryBatch(ctx, []int64{1, 2}, "2026-09-01", "2026-09-01", true); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Pool.Exec(ctx, `update summaries set content='stale', updated_at=now()-interval '1 hour' where chat_id=1 and summary_date='2026-09-01';
 update summaries set status='running' where chat_id=2 and summary_date='2026-09-01';
 update summary_batch_items set started_at=now()`); err != nil {
		t.Fatal(err)
	}
	runBatchRound("interrupted overwrite", func(item store.SummaryBatchItem) bool { return item.Status == "queued" && !item.Started })
	runBatchRound("overwrite rerun", func(item store.SummaryBatchItem) bool { return item.Status == "succeeded" })
	if rerun, err = st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-01"); err != nil || rerun.Content != "batch-content" {
		t.Fatalf("interrupted overwrite not regenerated: %+v %v", rerun, err)
	}

	// 不重新生成时，开始后未被更新的旧失败摘要同样说明生成前进程中断，重新生成而不是沿用旧错误。
	if _, err = service.QueueSummaryBatch(ctx, []int64{2}, "2026-09-02", "2026-09-02", false); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Pool.Exec(ctx, `update summaries set error_message='stale error', updated_at=now()-interval '1 hour' where chat_id=2 and summary_date='2026-09-02';
 update summary_batch_items set started_at=now()`); err != nil {
		t.Fatal(err)
	}
	runBatchRound("interrupted retry-free item", func(item store.SummaryBatchItem) bool { return item.Status == "queued" && !item.Started })
	runBatchRound("rerun failed item", func(item store.SummaryBatchItem) bool { return item.Status == "failed" && item.Error != "stale error" })
}
