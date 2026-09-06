package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DailyDigestRepository struct {
	pool *pgxpool.Pool
}

// Create 固化参与群组及其单群摘要，并保证同一日期只有一个任务。
func (r *DailyDigestRepository) Create(
	ctx context.Context,
	summaryDate string,
	sources []model.DailyDigestSource,
) (model.DailyDigest, bool, error) {
	sources = sortedDailyDigestSources(sources)
	participantCount, sourceCount, emptyCount, omittedCount := dailyDigestSourceCounts(sources)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.DailyDigest{}, false, fmt.Errorf("begin daily digest creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item := model.DailyDigest{
		SummaryDate: summaryDate, Status: model.SummaryStatusPending,
		ParticipantCount: participantCount, SourceSummaryCount: sourceCount,
		EmptyChatCount: emptyCount, OmittedChatCount: omittedCount,
	}
	err = tx.QueryRow(ctx, `
		insert into daily_digests (
			summary_date, participant_count, source_summary_count, empty_chat_count, omitted_chat_count
		)
		values ($1::date, $2, $3, $4, $5)
		on conflict (summary_date) do nothing
		returning id, created_at, updated_at
	`, summaryDate, participantCount, sourceCount, emptyCount, omittedCount).Scan(
		&item.ID, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return r.getExistingAfterConflict(ctx, summaryDate)
	}
	if err != nil {
		return model.DailyDigest{}, false, fmt.Errorf("insert daily digest for %s: %w", summaryDate, err)
	}
	if err := insertDailyDigestSources(ctx, tx, item.ID, sources); err != nil {
		return model.DailyDigest{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.DailyDigest{}, false, fmt.Errorf("commit daily digest for %s: %w", summaryDate, err)
	}
	item.Sources = sources
	return item, true, nil
}

// getExistingAfterConflict 返回并发创建者已经提交的同日任务。
func (r *DailyDigestRepository) getExistingAfterConflict(ctx context.Context, summaryDate string) (model.DailyDigest, bool, error) {
	item, err := r.GetByDate(ctx, summaryDate)
	if err != nil {
		return model.DailyDigest{}, false, err
	}
	return item, false, nil
}

// PrepareRegeneration 将来源快照与待生成状态一起提交，避免新来源配上旧正文和发送记录。
func (r *DailyDigestRepository) PrepareRegeneration(
	ctx context.Context,
	id int64,
	sources []model.DailyDigestSource,
) error {
	sources = sortedDailyDigestSources(sources)
	participantCount, sourceCount, emptyCount, omittedCount := dailyDigestSourceCounts(sources)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin daily digest source replacement: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `delete from daily_digest_sources where daily_digest_id = $1`, id); err != nil {
		return fmt.Errorf("delete daily digest %d sources: %w", id, err)
	}
	if err := insertDailyDigestSources(ctx, tx, id, sources); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update daily_digests
		set participant_count = $1, source_summary_count = $2, empty_chat_count = $3,
		    omitted_chat_count = $4, updated_at = now(),
		    status = 'pending', content = '', model = '', chunk_count = 0,
		    execution_mode = '', estimated_input_tokens = 0, context_window_tokens = 0,
		    fallback_reason = '', delivery_skipped_reason = '', delivery_suppressed = false,
		    delivered_at = null, delivery_error = '', error_message = '', retry_count = 0,
		    next_retry_at = null, generated_at = null, completed_at = null
		where id = $5
	`, participantCount, sourceCount, emptyCount, omittedCount, id); err != nil {
		return fmt.Errorf("update daily digest %d source counts: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit daily digest %d source replacement: %w", id, err)
	}
	return nil
}

// GetByID 返回每日总览详情及来源元数据。
func (r *DailyDigestRepository) GetByID(ctx context.Context, id int64) (model.DailyDigest, error) {
	return r.getByID(ctx, id, false)
}

// GetForGeneration 返回生成阶段需要的来源正文。
func (r *DailyDigestRepository) GetForGeneration(ctx context.Context, id int64) (model.DailyDigest, error) {
	return r.getByID(ctx, id, true)
}

// getByID 仅在模型生成场景加载可能很大的来源正文。
func (r *DailyDigestRepository) getByID(ctx context.Context, id int64, includeContent bool) (model.DailyDigest, error) {
	item, err := scanDailyDigestRow(r.pool.QueryRow(ctx, dailyDigestSelect+` where id = $1`, id))
	if err != nil {
		return model.DailyDigest{}, fmt.Errorf("get daily digest %d: %w", id, err)
	}
	item.Sources, err = r.listSources(ctx, id, includeContent)
	if err != nil {
		return model.DailyDigest{}, err
	}
	return item, nil
}

// GetByDate 返回指定摘要日期的每日总览。
func (r *DailyDigestRepository) GetByDate(ctx context.Context, summaryDate string) (model.DailyDigest, error) {
	item, err := scanDailyDigestRow(r.pool.QueryRow(ctx, dailyDigestSelect+` where summary_date = $1::date`, summaryDate))
	if err != nil {
		return model.DailyDigest{}, fmt.Errorf("get daily digest for %s: %w", summaryDate, err)
	}
	return item, nil
}

// List 按摘要日期倒序返回每日总览历史。
func (r *DailyDigestRepository) List(ctx context.Context, page int, pageSize int) (model.DailyDigestListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}
	var total int
	if err := r.pool.QueryRow(ctx, `select count(*) from daily_digests`).Scan(&total); err != nil {
		return model.DailyDigestListResponse{}, fmt.Errorf("count daily digests: %w", err)
	}
	rows, err := r.pool.Query(
		ctx,
		dailyDigestSelect+` order by summary_date desc, id desc limit $1 offset $2`,
		pageSize,
		(page-1)*pageSize,
	)
	if err != nil {
		return model.DailyDigestListResponse{}, fmt.Errorf("list daily digests: %w", err)
	}
	defer rows.Close()

	items := make([]model.DailyDigest, 0)
	for rows.Next() {
		item, err := scanDailyDigestRow(rows)
		if err != nil {
			return model.DailyDigestListResponse{}, fmt.Errorf("scan daily digest: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.DailyDigestListResponse{}, fmt.Errorf("iterate daily digests: %w", err)
	}
	return model.DailyDigestListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// SetRunning 清空已有结果，并记录任务完成后是否只允许手动发送。
func (r *DailyDigestRepository) SetRunning(ctx context.Context, id int64, suppressDelivery bool) error {
	_, err := r.pool.Exec(ctx, `
		update daily_digests
		set status = 'running', content = '', model = '', chunk_count = 0,
		    execution_mode = '', estimated_input_tokens = 0, context_window_tokens = 0,
		    fallback_reason = '', delivery_skipped_reason = '', delivery_suppressed = $2,
		    delivered_at = null,
		    delivery_error = '', error_message = '', retry_count = 0, next_retry_at = null,
		    generated_at = null, completed_at = null, updated_at = now()
		where id = $1
	`, id, suppressDelivery)
	if err != nil {
		return fmt.Errorf("set daily digest %d running: %w", id, err)
	}
	return nil
}

// SetRetryRunning 开始一次自动重试并递增已执行的重试次数。
func (r *DailyDigestRepository) SetRetryRunning(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		update daily_digests
		set status = 'running', error_message = '', retry_count = retry_count + 1,
		    next_retry_at = null, updated_at = now()
		where id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("set daily digest %d retry running: %w", id, err)
	}
	return nil
}

// SaveResult 保存生成结果和下一次自动重试时间。
func (r *DailyDigestRepository) SaveResult(ctx context.Context, item model.DailyDigest) error {
	_, err := r.pool.Exec(ctx, `
		update daily_digests
		set status = $1, content = $2, model = $3, chunk_count = $4,
		    execution_mode = $5, estimated_input_tokens = $6, context_window_tokens = $7,
		    fallback_reason = $8, delivery_skipped_reason = $9, error_message = $10,
		    next_retry_at = $11, generated_at = $12, completed_at = $13, updated_at = now()
		where id = $14
	`,
		item.Status, item.Content, item.Model, item.ChunkCount,
		item.ExecutionMode, item.EstimatedInputTokens, item.ContextWindowTokens,
		item.FallbackReason, item.DeliverySkippedReason, item.ErrorMessage,
		item.NextRetryAt, item.GeneratedAt, item.CompletedAt, item.ID,
	)
	if err != nil {
		return fmt.Errorf("save daily digest %d result: %w", item.ID, err)
	}
	return nil
}

// MarkDelivered 记录每日总览已发送到 Telegram。
func (r *DailyDigestRepository) MarkDelivered(ctx context.Context, id int64, deliveredAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update daily_digests
		set delivered_at = $1, delivery_error = '', updated_at = now()
		where id = $2
	`, deliveredAt, id)
	if err != nil {
		return fmt.Errorf("mark daily digest %d delivered: %w", id, err)
	}
	return nil
}

// MarkDeliveryFailed 保留正文并记录 Telegram 投递错误。
func (r *DailyDigestRepository) MarkDeliveryFailed(ctx context.Context, id int64, message string) error {
	_, err := r.pool.Exec(ctx, `
		update daily_digests
		set delivered_at = null, delivery_error = $1, updated_at = now()
		where id = $2
	`, message, id)
	if err != nil {
		return fmt.Errorf("mark daily digest %d delivery failed: %w", id, err)
	}
	return nil
}

// RecoverInterrupted 将进程退出时中断的任务标记为可重试失败。
func (r *DailyDigestRepository) RecoverInterrupted(ctx context.Context, message string) error {
	_, err := r.pool.Exec(ctx, `
		update daily_digests
		set status = 'failed', error_message = $1, next_retry_at = now(),
		    completed_at = now(), updated_at = now()
		where status = 'running'
	`, message)
	if err != nil {
		return fmt.Errorf("recover interrupted daily digests: %w", err)
	}
	return nil
}

const dailyDigestSelect = `
	select id, summary_date::text, status, content, model, participant_count,
	       source_summary_count, empty_chat_count, omitted_chat_count, chunk_count,
	       execution_mode, estimated_input_tokens, context_window_tokens, fallback_reason,
	       delivery_skipped_reason, delivery_suppressed, delivered_at, delivery_error, error_message,
	       retry_count, next_retry_at, generated_at, completed_at, created_at, updated_at
	from daily_digests`

type dailyDigestScanner interface {
	Scan(dest ...any) error
}

// scanDailyDigestRow 与 dailyDigestSelect 的固定列顺序保持一致。
func scanDailyDigestRow(scanner dailyDigestScanner) (model.DailyDigest, error) {
	var item model.DailyDigest
	err := scanner.Scan(
		&item.ID, &item.SummaryDate, &item.Status, &item.Content, &item.Model,
		&item.ParticipantCount, &item.SourceSummaryCount, &item.EmptyChatCount,
		&item.OmittedChatCount, &item.ChunkCount, &item.ExecutionMode,
		&item.EstimatedInputTokens, &item.ContextWindowTokens, &item.FallbackReason,
		&item.DeliverySkippedReason, &item.DeliverySuppressed, &item.DeliveredAt, &item.DeliveryError,
		&item.ErrorMessage, &item.RetryCount, &item.NextRetryAt, &item.GeneratedAt,
		&item.CompletedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

// listSources 始终返回来源元数据，仅在生成阶段读取正文和模型快照。
func (r *DailyDigestRepository) listSources(ctx context.Context, id int64, includeContent bool) ([]model.DailyDigestSource, error) {
	contentColumn := "''::text"
	modelColumn := "''::text"
	if includeContent {
		contentColumn = "content"
		modelColumn = "model"
	}
	rows, err := r.pool.Query(ctx, `
		select summary_id, chat_id, chat_title, summary_status, source_message_count,
		       included, omission_reason, `+contentColumn+`, `+modelColumn+`
		from daily_digest_sources
		where daily_digest_id = $1
		order by chat_title, chat_id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("list daily digest %d sources: %w", id, err)
	}
	defer rows.Close()
	items := make([]model.DailyDigestSource, 0)
	for rows.Next() {
		var item model.DailyDigestSource
		if err := rows.Scan(
			&item.SummaryID, &item.ChatID, &item.ChatTitle, &item.SummaryStatus,
			&item.SourceMessageCount, &item.Included, &item.OmissionReason,
			&item.Content, &item.Model,
		); err != nil {
			return nil, fmt.Errorf("scan daily digest %d source: %w", id, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// insertDailyDigestSources 批量写入每日总览的群组与正文快照。
func insertDailyDigestSources(ctx context.Context, tx pgx.Tx, id int64, sources []model.DailyDigestSource) error {
	rows := make([][]any, 0, len(sources))
	for _, source := range sources {
		rows = append(rows, []any{
			id, source.SummaryID, source.ChatID, source.ChatTitle, source.SummaryStatus,
			source.SourceMessageCount, source.Included, source.OmissionReason,
			source.Content, source.Model,
		})
	}
	copied, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"daily_digest_sources"},
		[]string{
			"daily_digest_id", "summary_id", "chat_id", "chat_title", "summary_status",
			"source_message_count", "included", "omission_reason", "content", "model",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("insert daily digest %d sources: %w", id, err)
	}
	if copied != int64(len(rows)) {
		return fmt.Errorf("insert daily digest %d sources: copied %d of %d rows", id, copied, len(rows))
	}
	return nil
}

// sortedDailyDigestSources 复制并排序输入，保证来源编号和界面顺序可重复。
func sortedDailyDigestSources(sources []model.DailyDigestSource) []model.DailyDigestSource {
	result := append([]model.DailyDigestSource(nil), sources...)
	sort.Slice(result, func(left, right int) bool {
		if result[left].ChatTitle == result[right].ChatTitle {
			return result[left].ChatID < result[right].ChatID
		}
		return result[left].ChatTitle < result[right].ChatTitle
	})
	return result
}

// dailyDigestSourceCounts 将群组快照归类为有效来源、空群和失败群组。
func dailyDigestSourceCounts(sources []model.DailyDigestSource) (int, int, int, int) {
	sourceCount := 0
	emptyCount := 0
	omittedCount := 0
	for _, source := range sources {
		switch {
		case source.Included:
			sourceCount++
		case source.OmissionReason == "no_messages":
			emptyCount++
		default:
			omittedCount++
		}
	}
	return len(sources), sourceCount, emptyCount, omittedCount
}
