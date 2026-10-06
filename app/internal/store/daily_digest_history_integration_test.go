package store

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/config"
)

// TestDailyDigestHistory 在隔离数据库验证历史窗口、成功状态、空正文过滤及日期顺序。
func TestDailyDigestHistory(t *testing.T) {
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
	schema := fmt.Sprintf("digest_history_test_%d", time.Now().UnixNano())
	if _, err := admin.Pool.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, "drop schema "+schema+" cascade")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema+",public")
	query.Set("timezone", "Pacific/Honolulu")
	u.RawQuery = query.Encode()
	st, err := Open(ctx, config.Config{DatabaseURL: u.String(), MasterKey: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := RunMigrations(ctx, st); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `insert into daily_digests (summary_date, status, content, execution_mode) values
		('2026-09-27', 'succeeded', '窗口之外', 'single'),
		('2026-09-28', 'succeeded', '窗口下界', 'single'),
		('2026-09-29', 'failed', '失败正文', 'single'),
		('2026-09-30', 'succeeded', '   ', 'single'),
		('2026-10-01', 'succeeded', '无消息固定正文', 'no_content'),
		('2026-10-02', 'pending', '等待正文', 'single'),
		('2026-10-03', 'running', '运行正文', 'single'),
		('2026-10-04', 'succeeded', '最近总览', 'single'),
		('2026-10-05', 'succeeded', '目标当天', 'single'),
		('2026-10-06', 'succeeded', '未来总览', 'single')`); err != nil {
		t.Fatal(err)
	}
	items, err := st.DailyDigests.ListHistory(ctx, "2026-10-05", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].SummaryDate != "2026-10-04" || items[0].Content != "最近总览" || items[1].SummaryDate != "2026-09-28" {
		t.Fatalf("unexpected history: %+v", items)
	}
	items, err = st.DailyDigests.ListHistory(ctx, "2026-09-27", 7)
	if err != nil || len(items) != 0 {
		t.Fatalf("unexpected empty history: %+v, %v", items, err)
	}
}
