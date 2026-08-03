package store

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListMessagesKeysetPaginationWithPostgres(t *testing.T) {
	databaseURL := os.Getenv("TGTLDR_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TGTLDR_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `
		create temporary table messages (
			chat_id bigint not null,
			telegram_message_id integer not null,
			telegram_sender_id bigint not null,
			sender_name text not null default '',
			sender_username text not null default '',
			text_content text not null default '',
			caption text not null default '',
			message_type text not null default 'text',
			media_kind text not null default '',
			reply_to_message_id integer not null default 0,
			message_time timestamptz not null
		)
	`)
	if err != nil {
		t.Fatalf("create temporary messages table: %v", err)
	}

	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		id       int
		senderID int64
		time     time.Time
	}{
		{id: 1, senderID: 42, time: base},
		{id: 2, senderID: 42, time: base},
		{id: 6, senderID: 43, time: base.Add(30 * time.Second)},
		{id: 3, senderID: 42, time: base.Add(time.Minute)},
		{id: 4, senderID: 42, time: base.Add(2 * time.Minute)},
		{id: 5, senderID: 42, time: base.Add(3 * time.Minute)},
	} {
		if _, err := pool.Exec(ctx, `
			insert into messages (chat_id, telegram_message_id, telegram_sender_id, text_content, message_time)
			values (1, $1, $2, $3, $4)
		`, row.id, row.senderID, "message", row.time); err != nil {
			t.Fatalf("insert message %d: %v", row.id, err)
		}
	}

	repository := &MessageRepository{pool: pool}
	senderID := int64(42)
	firstPage, err := repository.ListMessages(ctx, 1, &senderID, nil, nil, nil, 0, 2)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	assertTelegramMessageIDs(t, firstPage, []int{1, 2})

	cursorTime := firstPage[len(firstPage)-1].MessageTime
	secondPage, err := repository.ListMessages(ctx, 1, &senderID, nil, nil, &cursorTime, 2, 2)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	assertTelegramMessageIDs(t, secondPage, []int{3, 4})

	to := base.Add(2 * time.Minute)
	bounded, err := repository.ListMessages(ctx, 1, &senderID, &base, &to, nil, 0, 10)
	if err != nil {
		t.Fatalf("bounded page: %v", err)
	}
	assertTelegramMessageIDs(t, bounded, []int{1, 2, 3})

	wholeChat, err := repository.ListMessages(ctx, 1, nil, nil, nil, nil, 0, 4)
	if err != nil {
		t.Fatalf("whole chat page: %v", err)
	}
	assertTelegramMessageIDs(t, wholeChat, []int{1, 2, 6, 3})
}

func assertTelegramMessageIDs(t *testing.T, messages []model.Message, want []int) {
	t.Helper()
	got := make([]int, 0, len(messages))
	for _, message := range messages {
		got = append(got, message.TelegramMessageID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("message IDs = %v, want %v", got, want)
	}
}
