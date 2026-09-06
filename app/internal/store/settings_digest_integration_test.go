package store

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// TestDigestActivation 使用专用测试数据库验证参与范围、重复保存和失败回滚。
func TestDigestActivation(t *testing.T) {
	dsn := os.Getenv("TGTLDR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("需要专用 TGTLDR_TEST_DATABASE_URL")
	}
	ctx := t.Context()
	admin, err := Open(ctx, config.Config{DatabaseURL: dsn, MasterKey: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("digest_test_%d", time.Now().UnixNano())
	if _, err := admin.Pool.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, "drop schema "+schema+" cascade")
	databaseURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := databaseURL.Query()
	query.Set("search_path", schema+",public")
	databaseURL.RawQuery = query.Encode()
	st, err := Open(ctx, config.Config{DatabaseURL: databaseURL.String(), MasterKey: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := RunMigrations(ctx, st); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `insert into chats (telegram_chat_id, title, summary_enabled, delivery_mode)
		values (900001, 'enabled', true, 'dashboard'), (900002, 'disabled', false, 'dashboard'), (900003, 'bot', true, 'bot')`); err != nil {
		t.Fatal(err)
	}
	settings, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.BotEnabled = true
	settings.BotSummaryDeliveryMode = model.BotSummaryDeliveryModeDailyDigest
	if _, err := st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := st.Pool.QueryRow(ctx, `select count(*) from chats where delivery_mode = 'bot'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 participants, got %d", count)
	}
	if _, err := st.Pool.Exec(ctx, `update chats set delivery_mode = 'dashboard' where telegram_chat_id = 900001`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `select count(*) from chats where delivery_mode = 'bot'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("ordinary save re-enabled an excluded chat")
	}
	settings.BotEnabled = false
	if _, err := st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `alter table chats add constraint test_block_delivery check (telegram_chat_id <> 900001 or delivery_mode <> 'bot')`); err != nil {
		t.Fatal(err)
	}
	settings.BotEnabled = true
	if _, err := st.Settings.Save(ctx, settings); err == nil {
		t.Fatal("expected participant update failure")
	}
	stored, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored.BotEnabled {
		t.Fatal("settings must roll back when participant update fails")
	}
	if _, err := st.Pool.Exec(ctx, `alter table chats drop constraint test_block_delivery`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `select count(*) from chats where delivery_mode = 'bot'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("re-enabling Bot in digest mode must include AI chats")
	}
}
