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

const chatSelectCols = `id, telegram_chat_id, telegram_access_hash, title, username, chat_type,
       enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
       delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
       alert_enabled, alert_keywords, summary_frequency,
       created_at, updated_at`

func scanChat(scanner interface{ Scan(...any) error }, chat *model.Chat) error {
	return scanner.Scan(
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
		&chat.SummaryFrequency,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
}

func (r *ChatRepository) List(ctx context.Context) ([]model.Chat, error) {
	rows, err := r.pool.Query(ctx, `
		select `+chatSelectCols+`
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
		if err := scanChat(rows, &chat); err != nil {
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

type NewChatDefaults struct {
	DeliveryMode     model.DeliveryMode
	SummaryTimeLocal string
	Timezone         string
	KeepBotMessages  bool
	ModelOverride    string
}

func (d NewChatDefaults) resolveDeliveryMode() string {
	if d.DeliveryMode != "" {
		return string(d.DeliveryMode)
	}
	return "dashboard"
}

func (d NewChatDefaults) resolveSummaryTimeLocal() string {
	if d.SummaryTimeLocal != "" {
		return d.SummaryTimeLocal
	}
	return "09:00"
}

func (d NewChatDefaults) resolveTimezone() string {
	if d.Timezone != "" {
		return d.Timezone
	}
	return "Asia/Shanghai"
}

func (r *ChatRepository) UpsertMany(ctx context.Context, chats []model.Chat, defaults NewChatDefaults) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chats tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, chat := range chats {
		_, err := tx.Exec(ctx, `
			insert into chats (
				telegram_chat_id, telegram_access_hash, title, username, chat_type,
				enabled, summary_enabled, summary_context, summary_prompt, summary_time_local, summary_timezone,
				delivery_mode, model_override, keep_bot_messages, filtered_senders, filtered_keywords,
				alert_enabled, alert_keywords, summary_frequency
			) values ($1, $2, $3, $4, $5, false, false, '', '', $6, $7, $8, $9, $10, '{}', '{}', false, '{}', 'daily')
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
			defaults.resolveSummaryTimeLocal(),
			defaults.resolveTimezone(),
			defaults.resolveDeliveryMode(),
			defaults.ModelOverride,
			defaults.KeepBotMessages,
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
	row := r.pool.QueryRow(ctx, `
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
		    summary_frequency = $13,
		    updated_at = now()
		where id = $14
		returning `+chatSelectCols,
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
		chat.SummaryFrequency,
		chat.ID,
	)
	if err := scanChat(row, &saved); err != nil {
		return model.Chat{}, fmt.Errorf("save chat %d: %w", chat.ID, err)
	}
	return saved, nil
}

func (r *ChatRepository) GetByID(ctx context.Context, id int64) (model.Chat, error) {
	var chat model.Chat
	row := r.pool.QueryRow(ctx, `
		select `+chatSelectCols+`
		from chats
		where id = $1
	`, id)
	if err := scanChat(row, &chat); err != nil {
		return model.Chat{}, fmt.Errorf("get chat %d: %w", id, err)
	}
	return chat, nil
}

func (r *ChatRepository) ListSummaryEnabled(ctx context.Context) ([]model.Chat, error) {
	rows, err := r.pool.Query(ctx, `
		select `+chatSelectCols+`
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
		if err := scanChat(rows, &chat); err != nil {
			return nil, fmt.Errorf("scan summary enabled chat: %w", err)
		}
		out = append(out, chat)
	}
	return out, rows.Err()
}

func (r *ChatRepository) ListAlertEnabled(ctx context.Context) ([]model.Chat, error) {
	rows, err := r.pool.Query(ctx, `
		select `+chatSelectCols+`
		from chats
		where enabled = true and alert_enabled = true and array_length(alert_keywords, 1) > 0
		order by id asc
	`)
	if err != nil {
		return nil, fmt.Errorf("query alert enabled chats: %w", err)
	}
	defer rows.Close()

	out := make([]model.Chat, 0)
	for rows.Next() {
		var chat model.Chat
		if err := scanChat(rows, &chat); err != nil {
			return nil, fmt.Errorf("scan alert enabled chat: %w", err)
		}
		out = append(out, chat)
	}
	return out, rows.Err()
}

func (r *ChatRepository) GetByTelegramID(ctx context.Context, telegramID int64) (model.Chat, error) {
	var chat model.Chat
	row := r.pool.QueryRow(ctx, `
		select `+chatSelectCols+`
		from chats
		where telegram_chat_id = $1
	`, telegramID)
	if err := scanChat(row, &chat); err != nil {
		return model.Chat{}, fmt.Errorf("get chat by telegram id %d: %w", telegramID, err)
	}
	return chat, nil
}

func (r *ChatRepository) EnsureExists(ctx context.Context, chat model.Chat, defaults NewChatDefaults) (model.Chat, error) {
	if err := r.UpsertMany(ctx, []model.Chat{chat}, defaults); err != nil {
		return model.Chat{}, err
	}
	return r.GetByTelegramID(ctx, chat.TelegramChatID)
}

func (r *ChatRepository) ApplyDefaultsToAll(ctx context.Context, defaults NewChatDefaults) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		update chats
		set delivery_mode      = $1,
		    summary_time_local = $2,
		    keep_bot_messages  = $3,
		    model_override     = $4,
		    updated_at         = now()
	`,
		defaults.resolveDeliveryMode(),
		defaults.resolveSummaryTimeLocal(),
		defaults.KeepBotMessages,
		defaults.ModelOverride,
	)
	if err != nil {
		return 0, fmt.Errorf("apply defaults to all chats: %w", err)
	}
	return tag.RowsAffected(), nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
