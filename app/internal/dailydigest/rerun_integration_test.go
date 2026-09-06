package dailydigest

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

type deliveryTransport func(*http.Request) (*http.Response, error)

func (f deliveryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestRerunLatestParticipants 在隔离数据库验证来源替换、未就绪保护和重建后的自动发送。
func TestRerunLatestParticipants(t *testing.T) {
	dsn := os.Getenv("TGTLDR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("需要专用 TGTLDR_TEST_DATABASE_URL")
	}
	ctx := t.Context()
	admin, err := store.Open(ctx, config.Config{DatabaseURL: dsn, MasterKey: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("rerun_test_%d", time.Now().UnixNano())
	if _, err := admin.Pool.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, "drop schema "+schema+" cascade")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	st, err := store.Open(ctx, config.Config{DatabaseURL: u.String(), MasterKey: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := store.RunMigrations(ctx, st); err != nil {
		t.Fatal(err)
	}
	settings, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.BotEnabled = true
	settings.BotToken = "test-token"
	settings.BotTargetChatID = "123"
	settings.BotSummaryDeliveryMode = model.BotSummaryDeliveryModeDailyDigest
	if _, err := st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `insert into chats (id, telegram_chat_id, title, summary_enabled, delivery_mode)
		overriding system value values (1, 1, '退出群', true, 'dashboard'), (2, 2, '加入群', true, 'bot'), (3, 3, '缺少摘要群', true, 'bot');
		insert into summaries (id, chat_id, summary_date, status, content, source_message_count, bot_summary_delivery_mode)
		overriding system value values (1, 1, '2026-01-01', 'succeeded', '原摘要', 1, 'daily_digest'),
		(2, 2, '2026-01-01', 'succeeded', '最新摘要', 2, 'daily_digest')`); err != nil {
		t.Fatal(err)
	}
	if err := st.Summaries.SaveResult(ctx, model.Summary{
		ChatID: 2, SummaryDate: "2026-01-01", Status: model.SummaryStatusSucceeded,
		Content: "最新摘要", SourceMessageCount: 2, GeneratedAt: time.Now(),
		Model: "terra", RequestedModel: "luna", ReturnedModel: "terra",
	}); err != nil {
		t.Fatal(err)
	}
	item, _, err := st.DailyDigests.Create(ctx, "2026-01-01", []model.DailyDigestSource{{ChatID: 1, SummaryID: 1, ChatTitle: "退出群", Included: true, Content: "原摘要", SummaryStatus: model.SummaryStatusSucceeded}})
	if err != nil {
		t.Fatal(err)
	}
	item.Status = model.SummaryStatusSucceeded
	item.Content = "原总览"
	if err := st.DailyDigests.SaveResult(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := st.DailyDigests.MarkDelivered(ctx, item.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	service := NewService(ctx, st, clock.System{}, bot.New(), time.Second)
	if err := service.Rerun(ctx, item.ID); !errors.Is(err, ErrSourcesNotReady) || !strings.Contains(err.Error(), "缺少摘要群") {
		t.Fatalf("missing source: %v", err)
	}
	unchanged, err := service.Get(ctx, item.ID)
	if err != nil || unchanged.Content != "原总览" || unchanged.DeliveredAt == nil {
		t.Fatalf("old digest changed: %+v, %v", unchanged, err)
	}
	if _, err := st.Pool.Exec(ctx, "update chats set summary_enabled = false where id = 3"); err != nil {
		t.Fatal(err)
	}
	sent := make(chan string, 1)
	previous := http.DefaultTransport
	http.DefaultTransport = deliveryTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		sent <- string(body)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	})
	defer func() { http.DefaultTransport = previous }()
	if err := service.Rerun(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case body := <-sent:
		if !strings.Contains(body, "最新摘要") || strings.Contains(body, "更新版") {
			t.Fatalf("unexpected message: %s", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("regeneration did not send")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		result, err := service.Get(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if result.DeliveredAt != nil && result.Content == "最新摘要" {
			if result.ParticipantCount != 1 || len(result.Sources) != 1 || result.Sources[0].ChatID != 2 || result.DeliverySuppressed {
				t.Fatalf("unexpected rebuilt sources: %+v", result)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("delivery was not recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}
	excluded, err := st.Summaries.GetByID(ctx, 1)
	if err != nil || excluded.DailyDigestID != 0 || excluded.DailyDigestStatus != model.SummaryStatusSucceeded || excluded.DailyDigestDeliveredAt != nil {
		t.Fatalf("excluded metadata: %+v, %v", excluded, err)
	}
	included, err := st.Summaries.GetByID(ctx, 2)
	if err != nil || included.DailyDigestID != item.ID || !included.DailyDigestIncluded {
		t.Fatalf("included metadata: %+v, %v", included, err)
	}
	if included.RequestedModel != "luna" || included.ReturnedModel != "terra" {
		t.Fatalf("model attribution lost: %+v", included)
	}
	list, err := st.Summaries.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range list {
		if summary.ChatID == 2 && (summary.RequestedModel != "luna" || summary.ReturnedModel != "terra") {
			t.Fatalf("list model attribution lost: %+v", summary)
		}
	}
}
