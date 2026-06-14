package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

func (r *ChatRepository) ListWithMessageActivity(
	ctx context.Context,
	startLocal time.Time,
	days int,
	timezone string,
) ([]model.Chat, error) {
	if days <= 0 {
		return r.List(ctx)
	}

	endLocal := startLocal.AddDate(0, 0, days)
	rows, err := r.pool.Query(ctx, `
		with daily_messages as (
			select
				chat_id,
				(message_time at time zone $3)::date as activity_date,
				count(*)::int as message_count
			from messages
			where message_time >= $1 and message_time < $2
			group by chat_id, activity_date
		),
		activity_by_chat as (
			select
				chat_id,
				jsonb_agg(
					jsonb_build_object(
						'date', activity_date::text,
						'messageCount', message_count
					)
					order by activity_date
				) as message_activity
			from daily_messages
			group by chat_id
		)
		select c.id, c.telegram_chat_id, c.telegram_access_hash, c.title, c.username, c.chat_type,
		       c.enabled, c.summary_enabled, c.summary_context, c.summary_prompt, c.summary_time_local, c.summary_timezone,
		       c.delivery_mode, c.model_override, c.keep_bot_messages, c.filtered_senders, c.filtered_keywords,
		       coalesce(a.message_activity, '[]'::jsonb),
		       c.created_at, c.updated_at
		from chats c
		left join activity_by_chat a on a.chat_id = c.id
		order by c.enabled desc, c.title asc
	`, startLocal.UTC(), endLocal.UTC(), timezone)
	if err != nil {
		return nil, fmt.Errorf("query chats with message activity: %w", err)
	}
	defer rows.Close()

	chats := make([]model.Chat, 0)
	for rows.Next() {
		chat, err := scanChatWithActivity(rows, startLocal, days)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

type chatActivityScanner interface {
	Scan(dest ...any) error
}

func scanChatWithActivity(scanner chatActivityScanner, startLocal time.Time, days int) (model.Chat, error) {
	var chat model.Chat
	var rawActivity []byte
	err := scanner.Scan(
		&chat.ID,
		&chat.TelegramChatID,
		&chat.TelegramAccess,
		&chat.Title,
		&chat.Username,
		&chat.ChatType,
		&chat.Enabled,
		&chat.SummaryEnabled,
		&chat.SummaryContext,
		&chat.SummaryPrompt,
		&chat.SummaryTimeLocal,
		&chat.SummaryTimezone,
		&chat.DeliveryMode,
		&chat.ModelOverride,
		&chat.KeepBotMessages,
		&chat.FilteredSenders,
		&chat.FilteredKeywords,
		&rawActivity,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		return model.Chat{}, fmt.Errorf("scan chat with message activity: %w", err)
	}

	var sparse []model.ChatMessageActivity
	if err := json.Unmarshal(rawActivity, &sparse); err != nil {
		return model.Chat{}, fmt.Errorf("decode chat message activity: %w", err)
	}
	chat.MessageActivity = completeMessageActivity(startLocal, days, sparse)
	return chat, nil
}

func completeMessageActivity(
	startLocal time.Time,
	days int,
	sparse []model.ChatMessageActivity,
) []model.ChatMessageActivity {
	counts := make(map[string]int, len(sparse))
	for _, item := range sparse {
		counts[item.Date] = item.MessageCount
	}

	out := make([]model.ChatMessageActivity, 0, days)
	for offset := 0; offset < days; offset++ {
		date := startLocal.AddDate(0, 0, offset).Format("2006-01-02")
		out = append(out, model.ChatMessageActivity{
			Date:         date,
			MessageCount: counts[date],
		})
	}
	return out
}
