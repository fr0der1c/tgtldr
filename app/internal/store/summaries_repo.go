package store

import (
	"context"
	"fmt"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SummaryRepository struct {
	pool *pgxpool.Pool
}

func (r *SummaryRepository) GetByID(ctx context.Context, id int64) (model.Summary, error) {
	var item model.Summary
	if err := scanSummary(r.pool.QueryRow(ctx, `
		select id, chat_id, summary_date::text, summary_type, window_start, window_end, status, content, model,
		       source_message_count, chunk_count, generated_at, delivered_at,
		       delivery_error, error_message, error_context, error_system_prompt,
		       error_user_prompt, retry_count, next_retry_at, ''::text as match_snippet,
		       '{}'::text[] as matched_fields, created_at, updated_at
		from summaries
		where id = $1
	`, id), &item); err != nil {
		return model.Summary{}, fmt.Errorf("get summary %d: %w", id, err)
	}
	return item, nil
}

func (r *SummaryRepository) GetByChatAndDate(ctx context.Context, chatID int64, date string) (model.Summary, error) {
	var item model.Summary
	if err := scanSummary(r.pool.QueryRow(ctx, `
		select id, chat_id, summary_date::text, summary_type, window_start, window_end, status, content, model,
		       source_message_count, chunk_count, generated_at, delivered_at,
		       delivery_error, error_message, error_context, error_system_prompt,
		       error_user_prompt, retry_count, next_retry_at, ''::text as match_snippet,
		       '{}'::text[] as matched_fields, created_at, updated_at
		from summaries
		where chat_id = $1 and summary_date = $2::date and summary_type = 'daily'
	`, chatID, date), &item); err != nil {
		return model.Summary{}, fmt.Errorf("get summary for chat %d on %s: %w", chatID, date, err)
	}
	return item, nil
}

func (r *SummaryRepository) List(ctx context.Context) ([]model.Summary, error) {
	result, err := r.Search(ctx, SummaryListParams{})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (r *SummaryRepository) Search(ctx context.Context, params SummaryListParams) (model.SummaryListResponse, error) {
	normalized := normalizeSummaryListParams(params)
	terms := searchTerms(normalized.Query)
	whereClause, args := buildSummaryWhereClause(normalized, terms)

	var total int
	countQuery := `
		select count(*)
		from summaries s
		join chats c on c.id = s.chat_id
	` + whereClause
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return model.SummaryListResponse{}, fmt.Errorf("count summaries: %w", err)
	}

	offset := (normalized.Page - 1) * normalized.PageSize
	argsWithPagination := append(args, normalized.PageSize, offset)
	dataQuery := `
		select s.id, s.chat_id, s.summary_date::text, s.summary_type, s.window_start, s.window_end, s.status, s.content, s.model,
		       s.source_message_count, s.chunk_count, s.generated_at, s.delivered_at,
		       s.delivery_error, s.error_message, s.error_context, s.error_system_prompt,
		       s.error_user_prompt, s.retry_count, s.next_retry_at, ''::text as match_snippet,
		       '{}'::text[] as matched_fields, s.created_at, s.updated_at, c.title
		from summaries s
		join chats c on c.id = s.chat_id
	` + whereClause + `
		order by s.summary_date desc, s.id desc
		limit $` + fmt.Sprint(len(args)+1) + ` offset $` + fmt.Sprint(len(args)+2)

	rows, err := r.pool.Query(ctx, dataQuery, argsWithPagination...)
	if err != nil {
		return model.SummaryListResponse{}, fmt.Errorf("query summaries: %w", err)
	}
	defer rows.Close()

	items := make([]model.Summary, 0)
	for rows.Next() {
		var item model.Summary
		var chatTitle string
		if err := scanSummaryWithChatTitle(rows, &item, &chatTitle); err != nil {
			return model.SummaryListResponse{}, fmt.Errorf("scan summary search result: %w", err)
		}
		item.MatchSnippet, item.MatchedFields = summarizeSearchMatch(item.Content, chatTitle, terms)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.SummaryListResponse{}, fmt.Errorf("iterate summaries: %w", err)
	}

	return model.SummaryListResponse{
		Items:    items,
		Total:    total,
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
	}, nil
}

func (r *SummaryRepository) UpsertPending(ctx context.Context, chatID int64, date string) error {
	_, err := r.pool.Exec(ctx, `
		insert into summaries (chat_id, summary_date, summary_type, status)
		values ($1, $2::date, 'daily', 'pending')
		on conflict (chat_id, summary_date, summary_type) where summary_type = 'daily' do nothing
	`, chatID, date)
	if err != nil {
		return fmt.Errorf("upsert pending summary: %w", err)
	}
	return nil
}

func (r *SummaryRepository) SetRunning(ctx context.Context, chatID int64, date string) error {
	_, err := r.pool.Exec(ctx, `
		update summaries
		set status = 'running',
		    error_message = '',
		    error_context = '',
		    error_system_prompt = '',
		    error_user_prompt = '',
		    retry_count = 0,
		    next_retry_at = null,
		    updated_at = now()
		where chat_id = $1 and summary_date = $2::date and summary_type = 'daily'
	`, chatID, date)
	if err != nil {
		return fmt.Errorf("set summary running: %w", err)
	}
	return nil
}

func (r *SummaryRepository) SetRetryRunning(ctx context.Context, chatID int64, date string) error {
	_, err := r.pool.Exec(ctx, `
		update summaries
		set status = 'running',
		    error_message = '',
		    error_context = '',
		    error_system_prompt = '',
		    error_user_prompt = '',
		    retry_count = retry_count + 1,
		    next_retry_at = null,
		    updated_at = now()
		where chat_id = $1 and summary_date = $2::date and summary_type = 'daily'
	`, chatID, date)
	if err != nil {
		return fmt.Errorf("set summary retry running: %w", err)
	}
	return nil
}

func (r *SummaryRepository) SaveResult(ctx context.Context, summary model.Summary) error {
	summaryType := normalizeSummaryType(summary.SummaryType)
	errorContext := summary.ErrorContext
	errorSystemPrompt := summary.ErrorSystemPrompt
	errorUserPrompt := summary.ErrorUserPrompt
	nextRetryAt := summary.NextRetryAt
	resetRetryCount := false
	if summary.Status == model.SummaryStatusSucceeded {
		errorContext = ""
		errorSystemPrompt = ""
		errorUserPrompt = ""
		nextRetryAt = nil
		resetRetryCount = true
	}
	_, err := r.pool.Exec(ctx, `
		update summaries
		set status = $1,
		    content = $2,
		    model = $3,
		    source_message_count = $4,
		    chunk_count = $5,
		    generated_at = $6,
		    window_start = $7,
		    window_end = $8,
		    error_message = $9,
		    error_context = $10,
		    error_system_prompt = $11,
		    error_user_prompt = $12,
		    retry_count = case when $13 then 0 else retry_count end,
		    next_retry_at = $14,
		    delivered_at = null,
		    delivery_error = '',
		    updated_at = now()
		where chat_id = $15
		  and summary_date = $16::date
		  and summary_type = $17
		  and ($17 = 'daily' or window_end = $18)
	`,
		summary.Status,
		summary.Content,
		summary.Model,
		summary.SourceMessageCount,
		summary.ChunkCount,
		summary.GeneratedAt,
		summary.WindowStart,
		summary.WindowEnd,
		summary.ErrorMessage,
		errorContext,
		errorSystemPrompt,
		errorUserPrompt,
		resetRetryCount,
		nextRetryAt,
		summary.ChatID,
		summary.SummaryDate,
		summaryType,
		summary.WindowEnd,
	)
	if err != nil {
		return fmt.Errorf("save summary result: %w", err)
	}
	return nil
}

func (r *SummaryRepository) ScheduleRetry(ctx context.Context, chatID int64, date string, nextRetryAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update summaries
		set next_retry_at = $1,
		    updated_at = now()
		where chat_id = $2 and summary_date = $3::date and summary_type = 'daily'
	`, nextRetryAt, chatID, date)
	if err != nil {
		return fmt.Errorf("schedule summary retry: %w", err)
	}
	return nil
}

func (r *SummaryRepository) MarkDelivered(ctx context.Context, chatID int64, date string, deliveredAt time.Time) error {
	return r.MarkSummaryDelivered(ctx, model.Summary{
		ChatID:      chatID,
		SummaryDate: date,
		SummaryType: model.SummaryTypeDaily,
	}, deliveredAt)
}

func (r *SummaryRepository) MarkSummaryDelivered(ctx context.Context, summary model.Summary, deliveredAt time.Time) error {
	summaryType := normalizeSummaryType(summary.SummaryType)
	_, err := r.pool.Exec(ctx, `
		update summaries
		set delivered_at = $1,
		    delivery_error = '',
		    updated_at = now()
		where chat_id = $2
		  and summary_date = $3::date
		  and summary_type = $4
		  and ($4 = 'daily' or window_end = $5)
	`, deliveredAt, summary.ChatID, summary.SummaryDate, summaryType, summary.WindowEnd)
	if err != nil {
		return fmt.Errorf("mark summary delivered: %w", err)
	}
	return nil
}

func (r *SummaryRepository) MarkDeliveryFailed(ctx context.Context, chatID int64, date string, message string) error {
	return r.MarkSummaryDeliveryFailed(ctx, model.Summary{
		ChatID:      chatID,
		SummaryDate: date,
		SummaryType: model.SummaryTypeDaily,
	}, message)
}

func (r *SummaryRepository) MarkSummaryDeliveryFailed(ctx context.Context, summary model.Summary, message string) error {
	summaryType := normalizeSummaryType(summary.SummaryType)
	_, err := r.pool.Exec(ctx, `
		update summaries
		set delivered_at = null,
		    delivery_error = $1,
		    updated_at = now()
		where chat_id = $2
		  and summary_date = $3::date
		  and summary_type = $4
		  and ($4 = 'daily' or window_end = $5)
	`, message, summary.ChatID, summary.SummaryDate, summaryType, summary.WindowEnd)
	if err != nil {
		return fmt.Errorf("mark summary delivery failed: %w", err)
	}
	return nil
}

func (r *SummaryRepository) SetFailed(ctx context.Context, chatID int64, date string, message string) error {
	_, err := r.pool.Exec(ctx, `
		update summaries
		set status = 'failed',
		    error_message = $1,
		    error_context = '',
		    error_system_prompt = '',
		    error_user_prompt = '',
		    next_retry_at = null,
		    updated_at = now()
		where chat_id = $2 and summary_date = $3::date and summary_type = 'daily'
	`, message, chatID, date)
	if err != nil {
		return fmt.Errorf("set summary failed: %w", err)
	}
	return nil
}

func (r *SummaryRepository) ExistsForDate(ctx context.Context, chatID int64, date string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		select exists(select 1 from summaries where chat_id = $1 and summary_date = $2::date and summary_type = 'daily' and status = 'succeeded')
	`, chatID, date).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check summary existence: %w", err)
	}
	return exists, nil
}

func (r *SummaryRepository) UpsertRollingPending(ctx context.Context, chatID int64, date string, windowStart, windowEnd time.Time) error {
	_, err := r.pool.Exec(ctx, `
		insert into summaries (chat_id, summary_date, summary_type, status, window_start, window_end)
		values ($1, $2::date, 'rolling', 'pending', $3, $4)
		on conflict (chat_id, summary_type, window_end) where summary_type = 'rolling' do nothing
	`, chatID, date, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("upsert rolling pending summary: %w", err)
	}
	return nil
}

func (r *SummaryRepository) SetRollingRunning(ctx context.Context, chatID int64, date string, windowEnd time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update summaries
		set status = 'running',
		    error_message = '',
		    error_context = '',
		    error_system_prompt = '',
		    error_user_prompt = '',
		    retry_count = 0,
		    next_retry_at = null,
		    updated_at = now()
		where chat_id = $1 and summary_date = $2::date and summary_type = 'rolling' and window_end = $3
	`, chatID, date, windowEnd)
	if err != nil {
		return fmt.Errorf("set rolling summary running: %w", err)
	}
	return nil
}

func (r *SummaryRepository) GetLatestRollingForDate(ctx context.Context, chatID int64, date string) (model.Summary, error) {
	var item model.Summary
	if err := scanSummary(r.pool.QueryRow(ctx, `
		select id, chat_id, summary_date::text, summary_type, window_start, window_end, status, content, model,
		       source_message_count, chunk_count, generated_at, delivered_at,
		       delivery_error, error_message, error_context, error_system_prompt,
		       error_user_prompt, retry_count, next_retry_at, ''::text as match_snippet,
		       '{}'::text[] as matched_fields, created_at, updated_at
		from summaries
		where chat_id = $1 and summary_date = $2::date and summary_type = 'rolling'
		order by window_end desc nulls last, id desc
		limit 1
	`, chatID, date), &item); err != nil {
		return model.Summary{}, fmt.Errorf("get latest rolling summary for chat %d on %s: %w", chatID, date, err)
	}
	return item, nil
}

func (r *SummaryRepository) CountRollingForDate(ctx context.Context, chatID int64, date string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		select count(*)
		from summaries
		where chat_id = $1
		  and summary_date = $2::date
		  and summary_type = 'rolling'
		  and status in ('pending', 'running', 'succeeded')
	`, chatID, date).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count rolling summaries for chat %d on %s: %w", chatID, date, err)
	}
	return count, nil
}

type summaryScanner interface {
	Scan(dest ...any) error
}

func normalizeSummaryType(summaryType model.SummaryType) model.SummaryType {
	if summaryType == model.SummaryTypeRolling {
		return model.SummaryTypeRolling
	}
	return model.SummaryTypeDaily
}

func scanSummary(scanner summaryScanner, item *model.Summary) error {
	return scanner.Scan(
		&item.ID,
		&item.ChatID,
		&item.SummaryDate,
		&item.SummaryType,
		&item.WindowStart,
		&item.WindowEnd,
		&item.Status,
		&item.Content,
		&item.Model,
		&item.SourceMessageCount,
		&item.ChunkCount,
		&item.GeneratedAt,
		&item.DeliveredAt,
		&item.DeliveryError,
		&item.ErrorMessage,
		&item.ErrorContext,
		&item.ErrorSystemPrompt,
		&item.ErrorUserPrompt,
		&item.RetryCount,
		&item.NextRetryAt,
		&item.MatchSnippet,
		&item.MatchedFields,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
}

func scanSummaryWithChatTitle(scanner summaryScanner, item *model.Summary, chatTitle *string) error {
	return scanner.Scan(
		&item.ID,
		&item.ChatID,
		&item.SummaryDate,
		&item.SummaryType,
		&item.WindowStart,
		&item.WindowEnd,
		&item.Status,
		&item.Content,
		&item.Model,
		&item.SourceMessageCount,
		&item.ChunkCount,
		&item.GeneratedAt,
		&item.DeliveredAt,
		&item.DeliveryError,
		&item.ErrorMessage,
		&item.ErrorContext,
		&item.ErrorSystemPrompt,
		&item.ErrorUserPrompt,
		&item.RetryCount,
		&item.NextRetryAt,
		&item.MatchSnippet,
		&item.MatchedFields,
		&item.CreatedAt,
		&item.UpdatedAt,
		chatTitle,
	)
}

func (r *SummaryRepository) PendingDueChats(ctx context.Context, now time.Time) ([]model.Chat, error) {
	rows, err := r.pool.Query(ctx, `
		select c.id, c.telegram_chat_id, c.telegram_access_hash, c.title, c.username, c.chat_type,
		       c.enabled, c.summary_prompt, c.summary_time_local, c.summary_timezone,
		       c.delivery_mode, c.created_at, c.updated_at
		from chats c
		where c.enabled = true
		order by c.id asc
	`)
	if err != nil {
		return nil, fmt.Errorf("query pending due chats: %w", err)
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
			&chat.SummaryPrompt,
			&chat.SummaryTimeLocal,
			&chat.SummaryTimezone,
			&chat.DeliveryMode,
			&chat.CreatedAt,
			&chat.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan due chat: %w", err)
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}
