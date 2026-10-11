package store

import (
	"testing"
	"time"
)

func TestAccountChatsSyncedAt(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	synced, err := st.Auth.Create(ctx, "+10000000001")
	if err != nil {
		t.Fatal(err)
	}
	unsynced, err := st.Auth.Create(ctx, "+10000000002")
	if err != nil {
		t.Fatal(err)
	}
	syncedAt := time.Date(2026, time.October, 11, 10, 0, 0, 0, time.UTC)
	if err = st.Auth.MarkChatsSynced(ctx, synced.ID, syncedAt); err != nil {
		t.Fatal(err)
	}

	accounts, err := st.Auth.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]*time.Time{}
	for _, account := range accounts {
		got[account.ID] = account.ChatsSyncedAt
	}
	if got[synced.ID] == nil || !got[synced.ID].Equal(syncedAt) {
		t.Fatalf("synced account: got %v, want %v", got[synced.ID], syncedAt)
	}
	if got[unsynced.ID] != nil {
		t.Fatalf("unsynced account: got %v, want nil", got[unsynced.ID])
	}
}

func TestChatFirstMessageDate(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	if _, err := st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title) overriding system value values(1,1,'a'),(2,2,'b')`); err != nil {
		t.Fatal(err)
	}
	// 2026-04-30 20:00 UTC 在 Asia/Shanghai 已是 2026-05-01。
	earliest := time.Date(2026, time.April, 30, 20, 0, 0, 0, time.UTC)
	if _, err := st.Pool.Exec(ctx, `insert into messages(chat_id,telegram_message_id,message_time) values
		(1,1,$1::timestamptz + interval '3 days'),(1,2,$1::timestamptz),(1,3,$1::timestamptz + interval '1 day')`, earliest); err != nil {
		t.Fatal(err)
	}

	chats, err := st.Chats.ListWithMessageActivity(ctx, time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), 30, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]string{}
	for _, chat := range chats {
		got[chat.ID] = chat.FirstMessageDate
	}
	if got[1] != "2026-05-01" {
		t.Fatalf("chat with messages: got %q, want 2026-05-01", got[1])
	}
	if got[2] != "" {
		t.Fatalf("chat without messages: got %q, want empty", got[2])
	}
}
