package scheduler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/dailydigest"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
)

type recoveryTransport func(*http.Request) (*http.Response, error)

func (f recoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// schedulerTestStore 使用独立 schema，所有 Bot 和模型调用由测试传输层截获。
func schedulerTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("TGTLDR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("需要专用 TGTLDR_TEST_DATABASE_URL")
	}
	cfg := config.Config{DatabaseURL: dsn, MasterKey: make([]byte, 32)}
	admin, err := store.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("recovery_worker_%d", time.Now().UnixNano())
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
	st, err := store.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err = store.RunMigrations(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestRecoveryWorker 验证一条失败不影响另一条、成功摘要不重写、重启后继续以及失败总览补发。
func TestRecoveryWorker(t *testing.T) {
	st := schedulerTestStore(t)
	ctx := t.Context()
	settings, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
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
	_, err = st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode) overriding system value values
 (1,1,'bad',true,'bot'),(2,2,'good',true,'bot'),(3,3,'preserved',true,'bot');
 insert into summaries(id,chat_id,summary_date,status,content,source_message_count,bot_summary_delivery_mode) overriding system value values
 (1,1,'2026-09-09','failed','',1,'daily_digest'),(2,2,'2026-09-09','failed','',1,'daily_digest'),(3,3,'2026-09-09','succeeded','preserved-content',1,'daily_digest');
 insert into messages(chat_id,telegram_message_id,text_content,message_time) values(1,1,'permanent-error','2026-09-09T10:00:00+08:00'),(2,1,'normal message','2026-09-09T10:00:00+08:00');
 insert into daily_digests(summary_date,status) values('2026-09-09','failed')`)
	if err != nil {
		t.Fatal(err)
	}
	var sent, modelCalls atomic.Int32
	oldTransport := http.DefaultTransport
	http.DefaultTransport = recoveryTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		code, result := 200, `{"model":"test-model","choices":[{"message":{"role":"assistant","content":"recovered-content"}}]}`
		if r.URL.Host == "api.telegram.org" {
			sent.Add(1)
			result = `{"ok":true}`
		} else {
			modelCalls.Add(1)
			if strings.Contains(string(body), "permanent-error") {
				code = 404
				result = `{"error":{"message":"model not found"}}`
			}
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(result)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	botService := bot.New()
	digests := dailydigest.NewService(ctx, st, clock.System{}, botService, time.Second)
	service := NewService(st, clock.System{}, summary.NewService(st, clock.System{}, time.Second), botService, digests)
	if err = st.QueueFailedSummaries(ctx); err != nil {
		t.Fatal(err)
	}
	if err = service.runRecovery(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := st.RecoveryItems(ctx)
	if err != nil || len(items) != 3 || items[0].Status != "failed" || items[1].Status != "succeeded" || items[2].Status != "queued" {
		t.Fatalf("summary round: %+v %v", items, err)
	}
	preserved, err := st.Summaries.GetByID(ctx, 3)
	if err != nil || preserved.Content != "preserved-content" {
		t.Fatal("successful summary overwritten", err)
	}
	if sent.Load() != 0 {
		t.Fatal("sent before digest generation")
	}
	// 新建服务模拟重启后重新读取持久化队列。
	restarted := NewService(st, clock.System{}, summary.NewService(st, clock.System{}, time.Second), botService, digests)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err = restarted.runRecovery(ctx); err != nil {
			t.Fatal(err)
		}
		items, err = st.RecoveryItems(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if items[2].Status != "queued" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("digest did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if items[2].Status != "succeeded" || sent.Load() != 1 || modelCalls.Load() != 3 {
		t.Fatalf("final: %+v sent=%d calls=%d", items, sent.Load(), modelCalls.Load())
	}
	if err = restarted.runRecovery(ctx); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 1 {
		t.Fatal("duplicate digest delivery")
	}
	if err = restarted.notifyFailures(ctx, "https://dashboard.test"); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 2 {
		t.Fatal("missing failure notice")
	}
	again := NewService(st, clock.System{}, nil, botService, digests)
	if err = again.notifyFailures(ctx, "https://dashboard.test"); err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 2 {
		t.Fatal("duplicate failure notice after restart")
	}
	// 旧日期的手动失败不能永远停留在待重试状态。
	if _, err = st.Pool.Exec(ctx, `update summaries set status='running',next_retry_at=null,retry_count=3 where id=1`); err != nil {
		t.Fatal(err)
	}
	if err = restarted.RecoverInterruptedSummaries(ctx); err != nil {
		t.Fatal(err)
	}
	if err = restarted.retryHistoricalFailures(ctx); err != nil {
		t.Fatal(err)
	}
	failed, err := st.Summaries.GetByID(ctx, 1)
	if err != nil || failed.RetryCount != 4 || failed.NextRetryAt != nil || failed.Status != model.SummaryStatusFailed {
		t.Fatalf("historical retry: %+v %v", failed, err)
	}
}
