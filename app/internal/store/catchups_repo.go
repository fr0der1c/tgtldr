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

var (
	ErrInvalidCatchUpChats = errors.New("some selected chats are unavailable for Catch Up")
	ErrNoCatchUpSources    = errors.New("no summaries are available for Catch Up")
)

type CatchUpRepository struct {
	pool *pgxpool.Pool
}

// Create 固化所选群组与每日摘要快照，确保生成期间来源内容不发生变化。
func (r *CatchUpRepository) Create(
	ctx context.Context,
	fromDate string,
	toDate string,
	chatIDs []int64,
	deliveryRequested bool,
) (model.CatchUp, error) {
	chatIDs = uniqueInt64s(chatIDs)
	if len(chatIDs) == 0 {
		return model.CatchUp{}, ErrInvalidCatchUpChats
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.CatchUp{}, fmt.Errorf("begin catch up creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	chats, err := selectCatchUpChats(ctx, tx, chatIDs)
	if err != nil {
		return model.CatchUp{}, err
	}
	if len(chats) != len(chatIDs) {
		return model.CatchUp{}, ErrInvalidCatchUpChats
	}
	sources, err := selectCatchUpSources(ctx, tx, chatIDs, fromDate, toDate)
	if err != nil {
		return model.CatchUp{}, err
	}
	if len(sources) == 0 {
		return model.CatchUp{}, ErrNoCatchUpSources
	}

	item, err := insertCatchUp(ctx, tx, fromDate, toDate, len(chats), len(sources), deliveryRequested)
	if err != nil {
		return model.CatchUp{}, err
	}
	if err := insertCatchUpSnapshots(ctx, tx, item.ID, chats, sources); err != nil {
		return model.CatchUp{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.CatchUp{}, fmt.Errorf("commit catch up creation: %w", err)
	}
	item.Chats = chats
	return item, nil
}

// GetByID 返回 Catch Up 详情及其群组和来源快照。
func (r *CatchUpRepository) GetByID(ctx context.Context, id int64) (model.CatchUp, error) {
	return r.getByID(ctx, id, false)
}

// GetForGeneration 返回生成所需的来源正文，避免详情轮询反复加载大文本。
func (r *CatchUpRepository) GetForGeneration(ctx context.Context, id int64) (model.CatchUp, error) {
	return r.getByID(ctx, id, true)
}

// getByID 按调用场景决定是否加载仅生成阶段需要的来源正文。
func (r *CatchUpRepository) getByID(ctx context.Context, id int64, includeSourceContent bool) (model.CatchUp, error) {
	item, err := scanCatchUpRow(r.pool.QueryRow(ctx, catchUpSelect+" where id = $1", id))
	if err != nil {
		return model.CatchUp{}, fmt.Errorf("get catch up %d: %w", id, err)
	}
	item.Chats, err = r.listChats(ctx, id)
	if err != nil {
		return model.CatchUp{}, err
	}
	item.Sources, err = r.listSources(ctx, id, includeSourceContent)
	if err != nil {
		return model.CatchUp{}, err
	}
	return item, nil
}

// List 按创建时间倒序返回 Catch Up 历史记录。
func (r *CatchUpRepository) List(ctx context.Context, page int, pageSize int) (model.CatchUpListResponse, error) {
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
	if err := r.pool.QueryRow(ctx, `select count(*) from catch_ups`).Scan(&total); err != nil {
		return model.CatchUpListResponse{}, fmt.Errorf("count catch ups: %w", err)
	}
	rows, err := r.pool.Query(ctx, catchUpSelect+` order by created_at desc, id desc limit $1 offset $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return model.CatchUpListResponse{}, fmt.Errorf("list catch ups: %w", err)
	}
	defer rows.Close()

	items := make([]model.CatchUp, 0)
	for rows.Next() {
		item, err := scanCatchUpRow(rows)
		if err != nil {
			return model.CatchUpListResponse{}, fmt.Errorf("scan catch up: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.CatchUpListResponse{}, fmt.Errorf("iterate catch ups: %w", err)
	}
	return model.CatchUpListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// SetRunning 清除上一次错误并把新任务标记为运行中。
func (r *CatchUpRepository) SetRunning(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		update catch_ups
		set status = 'running', error_message = '', completed_at = null, updated_at = now()
		where id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("set catch up %d running: %w", id, err)
	}
	return nil
}

// SaveResult 持久化生成结果；失败结果不会清除已固化的来源快照。
func (r *CatchUpRepository) SaveResult(ctx context.Context, item model.CatchUp) error {
	_, err := r.pool.Exec(ctx, `
		update catch_ups
		set status = $1, content = $2, model = $3, chunk_count = $4,
		    execution_mode = $5, estimated_input_tokens = $6, context_window_tokens = $7,
		    fallback_reason = $8, error_message = $9, generated_at = $10,
		    completed_at = $11, updated_at = now()
		where id = $12
	`,
		item.Status, item.Content, item.Model, item.ChunkCount,
		item.ExecutionMode, item.EstimatedInputTokens, item.ContextWindowTokens,
		item.FallbackReason, item.ErrorMessage, item.GeneratedAt,
		item.CompletedAt, item.ID,
	)
	if err != nil {
		return fmt.Errorf("save catch up %d result: %w", item.ID, err)
	}
	return nil
}

// MarkDelivered 记录完整 Catch Up 已发送到 Telegram。
func (r *CatchUpRepository) MarkDelivered(ctx context.Context, id int64, deliveredAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update catch_ups
		set delivered_at = $1, delivery_error = '', updated_at = now()
		where id = $2
	`, deliveredAt, id)
	if err != nil {
		return fmt.Errorf("mark catch up %d delivered: %w", id, err)
	}
	return nil
}

// MarkDeliveryFailed 保留生成成功状态，仅记录独立的投递错误。
func (r *CatchUpRepository) MarkDeliveryFailed(ctx context.Context, id int64, message string) error {
	_, err := r.pool.Exec(ctx, `
		update catch_ups
		set delivered_at = null, delivery_error = $1, updated_at = now()
		where id = $2
	`, message, id)
	if err != nil {
		return fmt.Errorf("mark catch up %d delivery failed: %w", id, err)
	}
	return nil
}

// FailInterrupted 将进程退出时未完成的任务标记为失败，避免历史列表永久显示运行中。
func (r *CatchUpRepository) FailInterrupted(ctx context.Context, message string) error {
	_, err := r.pool.Exec(ctx, `
		update catch_ups
		set status = 'failed', error_message = $1, completed_at = now(), updated_at = now()
		where status in ('pending', 'running')
	`, message)
	if err != nil {
		return fmt.Errorf("fail interrupted catch ups: %w", err)
	}
	return nil
}

const catchUpSelect = `
	select id, from_date::text, to_date::text, status, content, model,
	       chat_count, source_summary_count, chunk_count, execution_mode,
	       estimated_input_tokens, context_window_tokens, fallback_reason,
	       delivery_requested, delivered_at, delivery_error, error_message,
	       generated_at, completed_at, created_at, updated_at
	from catch_ups`

type catchUpScanner interface {
	Scan(dest ...any) error
}

// scanCatchUpRow 与 catchUpSelect 的固定列顺序保持一致。
func scanCatchUpRow(scanner catchUpScanner) (model.CatchUp, error) {
	var item model.CatchUp
	err := scanner.Scan(
		&item.ID, &item.FromDate, &item.ToDate, &item.Status, &item.Content, &item.Model,
		&item.ChatCount, &item.SourceSummaryCount, &item.ChunkCount, &item.ExecutionMode,
		&item.EstimatedInputTokens, &item.ContextWindowTokens, &item.FallbackReason,
		&item.DeliveryRequested, &item.DeliveredAt, &item.DeliveryError, &item.ErrorMessage,
		&item.GeneratedAt, &item.CompletedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

// selectCatchUpChats 只接受当前仍启用 AI 摘要的群组。
func selectCatchUpChats(ctx context.Context, tx pgx.Tx, chatIDs []int64) ([]model.CatchUpChat, error) {
	rows, err := tx.Query(ctx, `
		select id, title
		from chats
		where id = any($1::bigint[]) and summary_enabled = true
		order by title, id
	`, chatIDs)
	if err != nil {
		return nil, fmt.Errorf("select catch up chats: %w", err)
	}
	defer rows.Close()

	items := make([]model.CatchUpChat, 0, len(chatIDs))
	for rows.Next() {
		var item model.CatchUpChat
		if err := rows.Scan(&item.ChatID, &item.ChatTitle); err != nil {
			return nil, fmt.Errorf("scan catch up chat: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// selectCatchUpSources 按日期、群组标题和摘要 ID 排序可用的成功摘要。
func selectCatchUpSources(ctx context.Context, tx pgx.Tx, chatIDs []int64, fromDate, toDate string) ([]model.CatchUpSource, error) {
	rows, err := tx.Query(ctx, `
		select s.id, s.chat_id, c.title, s.summary_date::text, s.content
		from summaries s
		join chats c on c.id = s.chat_id
		where s.chat_id = any($1::bigint[])
		  and s.summary_date between $2::date and $3::date
		  and s.status = 'succeeded'
		  and s.source_message_count > 0
		  and btrim(s.content) <> ''
		order by s.summary_date, c.title, s.id
	`, chatIDs, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("select catch up sources: %w", err)
	}
	defer rows.Close()

	items := make([]model.CatchUpSource, 0)
	for rows.Next() {
		var item model.CatchUpSource
		if err := rows.Scan(&item.SummaryID, &item.ChatID, &item.ChatTitle, &item.SummaryDate, &item.Content); err != nil {
			return nil, fmt.Errorf("scan catch up source: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// insertCatchUp 创建父任务并记录选择范围的统计快照。
func insertCatchUp(
	ctx context.Context,
	tx pgx.Tx,
	fromDate string,
	toDate string,
	chatCount int,
	sourceCount int,
	deliveryRequested bool,
) (model.CatchUp, error) {
	item := model.CatchUp{
		FromDate: fromDate, ToDate: toDate, Status: model.SummaryStatusPending,
		ChatCount: chatCount, SourceSummaryCount: sourceCount, DeliveryRequested: deliveryRequested,
	}
	err := tx.QueryRow(ctx, `
		insert into catch_ups (from_date, to_date, chat_count, source_summary_count, delivery_requested)
		values ($1::date, $2::date, $3, $4, $5)
		returning id, created_at, updated_at
	`, fromDate, toDate, chatCount, sourceCount, deliveryRequested).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.CatchUp{}, fmt.Errorf("insert catch up: %w", err)
	}
	return item, nil
}

// insertCatchUpSnapshots 批量写入群组和摘要快照，来源正文随后不再依赖原摘要。
func insertCatchUpSnapshots(ctx context.Context, tx pgx.Tx, catchUpID int64, chats []model.CatchUpChat, sources []model.CatchUpSource) error {
	counts := make(map[int64]int, len(chats))
	for _, source := range sources {
		counts[source.ChatID]++
	}
	chatRows := make([][]any, 0, len(chats))
	for index := range chats {
		chats[index].SourceSummaryCount = counts[chats[index].ChatID]
		chatRows = append(chatRows, []any{
			catchUpID, chats[index].ChatID, chats[index].ChatTitle, chats[index].SourceSummaryCount,
		})
	}
	copied, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"catch_up_chats"},
		[]string{"catch_up_id", "chat_id", "chat_title", "source_summary_count"},
		pgx.CopyFromRows(chatRows),
	)
	if err != nil {
		return fmt.Errorf("insert catch up chat snapshots: %w", err)
	}
	if copied != int64(len(chatRows)) {
		return fmt.Errorf("insert catch up chat snapshots: copied %d of %d rows", copied, len(chatRows))
	}

	sourceRows := make([][]any, 0, len(sources))
	for _, source := range sources {
		summaryDate, err := time.Parse("2006-01-02", source.SummaryDate)
		if err != nil {
			return fmt.Errorf("parse catch up source date %s: %w", source.SummaryDate, err)
		}
		sourceRows = append(sourceRows, []any{
			catchUpID, source.SummaryID, source.ChatID, source.ChatTitle, summaryDate, source.Content,
		})
	}
	copied, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"catch_up_sources"},
		[]string{"catch_up_id", "summary_id", "chat_id", "chat_title", "summary_date", "content"},
		pgx.CopyFromRows(sourceRows),
	)
	if err != nil {
		return fmt.Errorf("insert catch up source snapshots: %w", err)
	}
	if copied != int64(len(sourceRows)) {
		return fmt.Errorf("insert catch up source snapshots: copied %d of %d rows", copied, len(sourceRows))
	}
	return nil
}

// listChats 返回任务创建时选择的群组及其实际来源数量。
func (r *CatchUpRepository) listChats(ctx context.Context, catchUpID int64) ([]model.CatchUpChat, error) {
	rows, err := r.pool.Query(ctx, `
		select chat_id, chat_title, source_summary_count
		from catch_up_chats where catch_up_id = $1 order by chat_title, chat_id
	`, catchUpID)
	if err != nil {
		return nil, fmt.Errorf("list catch up chats: %w", err)
	}
	defer rows.Close()
	items := make([]model.CatchUpChat, 0)
	for rows.Next() {
		var item model.CatchUpChat
		if err := rows.Scan(&item.ChatID, &item.ChatTitle, &item.SourceSummaryCount); err != nil {
			return nil, fmt.Errorf("scan catch up chat snapshot: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// listSources 始终返回来源元数据，仅在生成阶段附带正文。
func (r *CatchUpRepository) listSources(ctx context.Context, catchUpID int64, includeContent bool) ([]model.CatchUpSource, error) {
	contentColumn := "''::text"
	if includeContent {
		contentColumn = "content"
	}
	rows, err := r.pool.Query(ctx, `
		select summary_id, chat_id, chat_title, summary_date::text, `+contentColumn+`
		from catch_up_sources
		where catch_up_id = $1
		order by summary_date, chat_title, summary_id
	`, catchUpID)
	if err != nil {
		return nil, fmt.Errorf("list catch up sources: %w", err)
	}
	defer rows.Close()
	items := make([]model.CatchUpSource, 0)
	for rows.Next() {
		var item model.CatchUpSource
		if err := rows.Scan(&item.SummaryID, &item.ChatID, &item.ChatTitle, &item.SummaryDate, &item.Content); err != nil {
			return nil, fmt.Errorf("scan catch up source snapshot: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// uniqueInt64s 过滤非法和重复 ID，并生成确定性顺序。
func uniqueInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}
