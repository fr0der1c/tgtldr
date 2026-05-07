package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func (r *ChatRepository) List(ctx context.Context) ([]model.Chat, error) {
	rows, err := r.pool.Query(ctx, `
		select id, telegram_chat_id, telegram_access_hash, title, username, chat_type,
		       enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
		       delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
		       alert_enabled, alert_keywords,
		       created_at, updated_at
		from chats
		order by enabled desc, title asc
	`)
	if err != nil {
		return nil, fmt.Errorf("query chats: %w", err)
	}
	defer rows.Close()

	chats := make([]model.Chat, 0)
	for rows.Next() {
		var chat model.Chat
		err := rows.Scan(
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
			&chat.AlertEnabled,
			&chat.AlertKeywords,
			&chat.CreatedAt,
			&chat.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (r *ChatRepository) CountEnabled(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `select count(*) from chats where enabled = true`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count enabled chats: %w", err)
	}
	return count, nil
}

func (r *ChatRepository) UpsertMany(ctx context.Context, chats []model.Chat) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chats tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, chat := range chats {
		_, err := tx.Exec(ctx, `
			insert into chats (
				telegram_chat_id, telegram_access_hash, title, username, chat_type,
				enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone, delivery_mode, model_override,
				keep_bot_messages, filtered_senders, filtered_keywords, alert_enabled, alert_keywords
			) values ($1, $2, $3, $4, $5, false, false, '', '', '09:00', 'Asia/Shanghai', 'dashboard', '', true, '{}', '{}', false, '{}')
			on conflict (telegram_chat_id) do update
			set telegram_access_hash = excluded.telegram_access_hash,
			    title = excluded.title,
			    username = excluded.username,
			    chat_type = excluded.chat_type,
			    updated_at = now()
		`,
			chat.TelegramChatID,
			chat.TelegramAccess,
			chat.Title,
			chat.Username,
			chat.ChatType,
		)
		if err != nil {
			return fmt.Errorf("upsert chat %d: %w", chat.TelegramChatID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit chats tx: %w", err)
	}
	return nil
}

func (r *ChatRepository) Save(ctx context.Context, chat model.Chat) (model.Chat, error) {
	var saved model.Chat
	err := r.pool.QueryRow(ctx, `
		update chats
		set enabled = $1,
		    summary_enabled = $2,
		    summary_context = $3,
		    summary_prompt = $4,
		    summary_time_local = $5,
		    delivery_mode = $6,
		    model_override = $7,
		    keep_bot_messages = $8,
		    filtered_senders = $9,
		    filtered_keywords = $10,
		    alert_enabled = $11,
		    alert_keywords = $12,
		    updated_at = now()
		where id = $13
		returning id, telegram_chat_id, telegram_access_hash, title, username, chat_type,
		          enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
		          delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
		          alert_enabled, alert_keywords,
		          created_at, updated_at
	`,
		chat.Enabled,
		chat.SummaryEnabled,
		chat.SummaryContext,
		chat.SummaryPrompt,
		chat.SummaryTimeLocal,
		chat.DeliveryMode,
		chat.ModelOverride,
		chat.KeepBotMessages,
		chat.FilteredSenders,
		chat.FilteredKeywords,
		chat.AlertEnabled,
		chat.AlertKeywords,
		chat.ID,
	).Scan(
		&saved.ID,
		&saved.TelegramChatID,
		&saved.TelegramAccess,
		&saved.Title,
		&saved.Username,
		&saved.ChatType,
		&saved.Enabled,
		&saved.SummaryEnabled,
		&saved.SummaryContext,
		&saved.SummaryPrompt,
		&saved.SummaryTimeLocal,
		&saved.SummaryTimezone,
		&saved.DeliveryMode,
		&saved.ModelOverride,
		&saved.KeepBotMessages,
		&saved.FilteredSenders,
		&saved.FilteredKeywords,
		&saved.AlertEnabled,
		&saved.AlertKeywords,
		&saved.CreatedAt,
		&saved.UpdatedAt,
	)
	if err != nil {
		return model.Chat{}, fmt.Errorf("save chat %d: %w", chat.ID, err)
	}
	return saved, nil
}

func (r *ChatRepository) GetByID(ctx context.Context, id int64) (model.Chat, error) {
	var chat model.Chat
	err := r.pool.QueryRow(ctx, `
		select id, telegram_chat_id, telegram_access_hash, title, username, chat_type,
		       enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
		       delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
		       alert_enabled, alert_keywords,
		       created_at, updated_at
		from chats
		where id = $1
	`, id).Scan(
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
		&chat.AlertEnabled,
		&chat.AlertKeywords,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		return model.Chat{}, fmt.Errorf("get chat %d: %w", id, err)
	}
	return chat, nil
}

func (r *ChatRepository) ListSummaryEnabled(ctx context.Context) ([]model.Chat, error) {
	rows, err := r.pool.Query(ctx, `
		select id, telegram_chat_id, telegram_access_hash, title, username, chat_type,
		       enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
		       delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
		       alert_enabled, alert_keywords,
		       created_at, updated_at
		from chats
		where summary_enabled = true
		order by id asc
	`)
	if err != nil {
		return nil, fmt.Errorf("query summary enabled chats: %w", err)
	}
	defer rows.Close()

	out := make([]model.Chat, 0)
	for rows.Next() {
		var chat model.Chat
		err := rows.Scan(
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
			&chat.AlertEnabled,
			&chat.AlertKeywords,
			&chat.CreatedAt,
			&chat.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan summary enabled chat: %w", err)
		}
		out = append(out, chat)
	}
	return out, rows.Err()
}

func (r *ChatRepository) GetByTelegramID(ctx context.Context, telegramID int64) (model.Chat, error) {
	var chat model.Chat
	err := r.pool.QueryRow(ctx, `
		select id, telegram_chat_id, telegram_access_hash, title, username, chat_type,
		       enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
		       delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
		       alert_enabled, alert_keywords,
		       created_at, updated_at
		from chats
		where telegram_chat_id = $1
	`, telegramID).Scan(
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
		&chat.AlertEnabled,
		&chat.AlertKeywords,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		return model.Chat{}, fmt.Errorf("get chat by telegram id %d: %w", telegramID, err)
	}
	return chat, nil
}

func (r *ChatRepository) EnsureExists(ctx context.Context, chat model.Chat) (model.Chat, error) {
	if err := r.UpsertMany(ctx, []model.Chat{chat}); err != nil {
		return model.Chat{}, err
	}
	return r.GetByTelegramID(ctx, chat.TelegramChatID)
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
