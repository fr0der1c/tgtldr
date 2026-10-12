package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrSummaryBatchActive 表示上一批批量生成尚未结束。
var ErrSummaryBatchActive = errors.New("summary batch is still running")

// SummaryBatchItem 是批量生成中的一个群组日期及其结果。
type SummaryBatchItem struct {
	ChatID    int64     `json:"chatId"`
	ChatTitle string    `json:"chatTitle"`
	Date      string    `json:"date"`
	Overwrite bool      `json:"-"`
	Started   bool      `json:"-"`
	StartedAt time.Time `json:"-"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
}

// QueueSummaryBatch 以新批次替换已结束的批次；上一批仍有排队项时拒绝，避免两批交错执行。
func (s *Store) QueueSummaryBatch(ctx context.Context, items []SummaryBatchItem) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin summary batch: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(221012)`); err != nil {
		return err
	}
	if err = ensureNoActiveSummaryBatch(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `delete from summary_batch_items`); err != nil {
		return err
	}
	for position, item := range items {
		if _, err = tx.Exec(ctx, `insert into summary_batch_items(chat_id, summary_date, position, overwrite) values ($1, $2::date, $3, $4)`,
			item.ChatID, item.Date, position, item.Overwrite); err != nil {
			return fmt.Errorf("queue summary batch item: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// EnsureNoActiveSummaryBatch 在上一批仍有排队项时返回 ErrSummaryBatchActive。
func (s *Store) EnsureNoActiveSummaryBatch(ctx context.Context) error {
	return ensureNoActiveSummaryBatch(ctx, s.Pool)
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ensureNoActiveSummaryBatch(ctx context.Context, db rowQuerier) error {
	var active bool
	if err := db.QueryRow(ctx, `select exists(select 1 from summary_batch_items where status = 'queued')`).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrSummaryBatchActive
	}
	return nil
}

// SummaryBatchItems 按提交顺序返回当前批次，供后台执行和网页进度共用。
func (s *Store) SummaryBatchItems(ctx context.Context) ([]SummaryBatchItem, error) {
	rows, err := s.Pool.Query(ctx, `select b.chat_id, c.title, b.summary_date::text, b.overwrite, b.started_at, b.status, b.error_message
 from summary_batch_items b join chats c on c.id = b.chat_id order by b.position`)
	if err != nil {
		return nil, fmt.Errorf("list summary batch: %w", err)
	}
	defer rows.Close()
	items := []SummaryBatchItem{}
	for rows.Next() {
		var item SummaryBatchItem
		var startedAt *time.Time
		if err := rows.Scan(&item.ChatID, &item.ChatTitle, &item.Date, &item.Overwrite, &startedAt, &item.Status, &item.Error); err != nil {
			return nil, err
		}
		if startedAt != nil {
			item.Started, item.StartedAt = true, *startedAt
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// StartSummaryBatchItem 在生成前记录开始时间，重启后据此等待既有重试而不是重新生成。
func (s *Store) StartSummaryBatchItem(ctx context.Context, chatID int64, date string) (time.Time, error) {
	var startedAt time.Time
	err := s.Pool.QueryRow(ctx, `update summary_batch_items set started_at = now() where chat_id = $1 and summary_date = $2::date returning started_at`,
		chatID, date).Scan(&startedAt)
	return startedAt, err
}

// ResetSummaryBatchItem 把生成被中断的项退回未开始，下一轮重新生成。
func (s *Store) ResetSummaryBatchItem(ctx context.Context, chatID int64, date string) error {
	_, err := s.Pool.Exec(ctx, `update summary_batch_items set started_at = null where chat_id = $1 and summary_date = $2::date`, chatID, date)
	return err
}

// FinishSummaryBatchItem 记录单项的最终结果。
func (s *Store) FinishSummaryBatchItem(ctx context.Context, chatID int64, date string, status string, message string) error {
	_, err := s.Pool.Exec(ctx, `update summary_batch_items set status = $3, error_message = $4 where chat_id = $1 and summary_date = $2::date`,
		chatID, date, status, message)
	return err
}
