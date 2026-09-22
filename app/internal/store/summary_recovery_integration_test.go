package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/jackc/pgx/v5"
)

// recoveryTestStore 在专用数据库的独立 schema 内验证真实迁移和队列事务。
func recoveryTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TGTLDR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("需要专用 TGTLDR_TEST_DATABASE_URL")
	}
	cfg := config.Config{DatabaseURL: dsn, MasterKey: make([]byte, 32)}
	admin, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("recovery_%d", time.Now().UnixNano())
	if _, err = admin.Pool.Exec(t.Context(), "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Pool.Exec(context.Background(), "drop schema "+schema+" cascade"); admin.Close() })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	cfg.DatabaseURL = u.String()
	st, err := Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err = RunMigrations(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestRecoveryPersistence 验证失败快照、重复提交和重启后的批次进度读取。
func TestRecoveryPersistence(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	_, err := st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode) overriding system value values(1,1,'test',true,'dashboard');
 insert into summaries(chat_id,summary_date,status) values(1,'2026-09-08','succeeded'),(1,'2026-09-09','failed');
 insert into daily_digests(summary_date,status) values('2026-09-08','succeeded'),('2026-09-09','failed')`)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = st.QueueFailedSummaries(ctx); err != nil {
			t.Fatal(err)
		}
	}
	items, err := st.RecoveryItems(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if items[0].Date != "2026-09-09" || items[0].Kind != "summary" {
		t.Fatal(items)
	}
	dates, err := st.FailureDates(ctx)
	if err != nil || len(dates) != 1 || dates[0] != "2026-09-09" {
		t.Fatalf("failure dates: %v %v", dates, err)
	}
	states, err := st.FailureStates(ctx, dates[0])
	if err != nil || len(states) != 2 || states[0].Kind != "summary" || states[1].Kind != "digest" {
		t.Fatalf("failure states: %+v %v", states, err)
	}
	if err = st.QueueFailureNotice(ctx, dates[0], "must wait for recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.ClaimFailureNotice(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("notified during recovery: %v", err)
	}
	items[0].Status = "succeeded"
	if err = st.SaveRecoveryItem(ctx, items[0]); err != nil {
		t.Fatal(err)
	}
	if err = st.QueueFailedSummaries(ctx); err != nil {
		t.Fatal(err)
	}
	restarted := &Store{Pool: st.Pool}
	items, err = restarted.RecoveryItems(ctx)
	if err != nil || items[0].Status != "succeeded" {
		t.Fatalf("restart: %+v %v", items, err)
	}
	if _, err = st.Pool.Exec(ctx, `insert into summaries(chat_id,summary_date,status) values(1,'2026-09-10','pending')`); err != nil {
		t.Fatal(err)
	}
	if err = st.RecoverInterruptedSummaries(ctx, "test restart"); err != nil {
		t.Fatal(err)
	}
	interrupted, err := st.Summaries.GetByChatAndDate(ctx, 1, "2026-09-10")
	if err != nil || interrupted.Status != "failed" || interrupted.NextRetryAt == nil {
		t.Fatalf("pending task recovery: %+v %v", interrupted, err)
	}
}

// TestFailureNoticePersistence 验证日期去重、成功后不再领取和发送失败的有限重试。
func TestFailureNoticePersistence(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	for range 2 {
		if err := st.QueueFailureNotice(ctx, "2026-09-09", "test"); err != nil {
			t.Fatal(err)
		}
	}
	item, err := st.ClaimFailureNotice(ctx)
	if err != nil || item.Attempts != 1 {
		t.Fatalf("claim: %+v %v", item, err)
	}
	if _, err = st.ClaimFailureNotice(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("duplicate claim: %v", err)
	}
	if err = st.FinishFailureNotice(ctx, item.Date, ""); err != nil {
		t.Fatal(err)
	}
	restarted := &Store{Pool: st.Pool}
	if err = restarted.QueueFailureNotice(ctx, item.Date, "repeat"); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.ClaimFailureNotice(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("restart duplicate: %v", err)
	}
	if err = st.QueueFailureNotice(ctx, "2026-09-10", "test"); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 4; attempt++ {
		item, err = st.ClaimFailureNotice(ctx)
		if err != nil || item.Attempts != attempt {
			t.Fatalf("attempt %d: %+v %v", attempt, item, err)
		}
		if err = st.FinishFailureNotice(ctx, item.Date, "delivery_failed"); err != nil {
			t.Fatal(err)
		}
		if _, err = st.Pool.Exec(ctx, `update summary_failure_notices set next_attempt_at=now()`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = st.ClaimFailureNotice(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("retry limit: %v", err)
	}
	notices, err := st.FailureNotices(ctx)
	if err != nil || len(notices) != 2 || notices[0].Error != "delivery_failed" {
		t.Fatalf("notices: %+v %v", notices, err)
	}
}

// TestHistoricalRetrySelection 验证跨日重试只领取到期且未耗尽的任务，并避开普通调度和批量队列。
func TestHistoricalRetrySelection(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	_, err := st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled) overriding system value values(1,1,'test',true);
	insert into summaries(chat_id,summary_date,status,retry_count,next_retry_at) values
	(1,'2026-09-09','failed',3,now()),(1,'2026-09-20','failed',0,now()),
	(1,'2026-09-10','failed',0,now()+interval '1 hour'),(1,'2026-09-11','failed',4,now());
	insert into daily_digests(summary_date,status,next_retry_at) values('2026-09-09','failed',now()),('2026-09-08','running',null)`)
	if err != nil {
		t.Fatal(err)
	}
	items, err := st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 4, true)
	if err != nil || len(items) != 1 || items[0].Kind != "summary" {
		t.Fatalf("active digest guard: %+v %v", items, err)
	}
	if _, err = st.Pool.Exec(ctx, `update daily_digests set status='succeeded' where status='running'`); err != nil {
		t.Fatal(err)
	}
	items, err = st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 4, true)
	if err != nil || len(items) != 2 {
		t.Fatalf("due historical tasks: %+v %v", items, err)
	}
	items, err = st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 4, false)
	if err != nil || len(items) != 1 || items[0].Kind != "summary" {
		t.Fatalf("disabled bot: %+v %v", items, err)
	}
	if err = st.QueueFailedSummaries(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = st.HistoricalRetries(ctx, time.Now(), "2026-09-20", 4, true)
	if err != nil || len(items) != 0 {
		t.Fatalf("batch overlap: %+v %v", items, err)
	}
}
