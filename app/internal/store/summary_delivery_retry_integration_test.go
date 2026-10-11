package store

import (
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// TestSummaryDeliveryRetryPersistence 验证投递失败计数、下一次重试时间及成功投递和重新生成后的清零。
func TestSummaryDeliveryRetryPersistence(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	_, err := st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode) overriding system value values(1,1,'test',true,'bot');
 insert into summaries(chat_id,summary_date,status) values(1,'2026-09-08','succeeded')`)
	if err != nil {
		t.Fatal(err)
	}

	next := time.Date(2026, time.September, 9, 1, 3, 0, 0, time.UTC)
	if err = st.Summaries.ScheduleDeliveryRetry(ctx, 1, "2026-09-08", "first failure", &next); err != nil {
		t.Fatal(err)
	}
	item, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-08")
	if err != nil || item.DeliveryRetryCount != 1 || item.NextDeliveryRetryAt == nil || !item.NextDeliveryRetryAt.Equal(next) || item.DeliveryError != "first failure" {
		t.Fatalf("first failure: %+v %v", item, err)
	}

	if err = st.Summaries.ScheduleDeliveryRetry(ctx, 1, "2026-09-08", "limit reached", nil); err != nil {
		t.Fatal(err)
	}
	item, err = st.Summaries.GetByID(ctx, item.ID)
	if err != nil || item.DeliveryRetryCount != 2 || item.NextDeliveryRetryAt != nil {
		t.Fatalf("limit reached: %+v %v", item, err)
	}

	if err = st.Summaries.MarkDelivered(ctx, 1, "2026-09-08", next); err != nil {
		t.Fatal(err)
	}
	item, err = st.Summaries.GetByID(ctx, item.ID)
	if err != nil || item.DeliveryRetryCount != 0 || item.NextDeliveryRetryAt != nil || item.DeliveredAt == nil {
		t.Fatalf("delivered: %+v %v", item, err)
	}

	if err = st.Summaries.ScheduleDeliveryRetry(ctx, 1, "2026-09-08", "failed again", &next); err != nil {
		t.Fatal(err)
	}
	if err = st.Summaries.SaveResult(ctx, model.Summary{ChatID: 1, SummaryDate: "2026-09-08", Status: model.SummaryStatusSucceeded, GeneratedAt: next}); err != nil {
		t.Fatal(err)
	}
	item, err = st.Summaries.GetByID(ctx, item.ID)
	if err != nil || item.DeliveryRetryCount != 0 || item.NextDeliveryRetryAt != nil {
		t.Fatalf("regenerated: %+v %v", item, err)
	}
}

// TestHistoricalDeliveryRetrySelection 验证跨日的投递重试由维护任务领取，并遵守上限、Bot 状态与每日总览模式。
func TestHistoricalDeliveryRetrySelection(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	_, err := st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,delivery_mode) overriding system value values(1,1,'bot','bot'),(2,2,'web','dashboard');
	insert into summaries(id,chat_id,summary_date,status,delivery_retry_count,next_delivery_retry_at,bot_summary_delivery_mode,delivered_at) overriding system value values
	(1,1,'2026-09-09','succeeded',1,now(),'per_chat',null),
	(2,1,'2026-09-20','succeeded',1,now(),'per_chat',null),
	(3,1,'2026-09-10','succeeded',1,now()+interval '1 hour','per_chat',null),
	(4,1,'2026-09-11','succeeded',5,now(),'per_chat',null),
	(5,1,'2026-09-12','succeeded',1,now(),'daily_digest',null),
	(6,1,'2026-09-13','succeeded',1,now(),'per_chat',now()),
	(7,2,'2026-09-14','succeeded',1,now(),'per_chat',null)`)
	if err != nil {
		t.Fatal(err)
	}
	items, err := st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 4, true)
	if err != nil || len(items) != 1 || items[0].Kind != "delivery" || items[0].ID != 1 {
		t.Fatalf("due delivery retries: %+v %v", items, err)
	}
	items, err = st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 4, false)
	if err != nil || len(items) != 0 {
		t.Fatalf("disabled bot: %+v %v", items, err)
	}
	items, err = st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 0, true)
	if err != nil || len(items) != 0 {
		t.Fatalf("disabled retries: %+v %v", items, err)
	}
}
