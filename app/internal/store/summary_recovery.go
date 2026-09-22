package store

import (
	"context"
	"fmt"
)

type RecoveryItem struct {
	Kind    string `json:"kind"`
	ID      int64  `json:"id"`
	Date    string `json:"date"`
	Started bool   `json:"started"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

// QueueFailedSummaries 在事务内固化失败任务快照，重复点击时保留正在执行的批次。
func (s *Store) QueueFailedSummaries(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(220921)`); err != nil {
		return err
	}
	var active bool
	if err = tx.QueryRow(ctx, `select exists(select 1 from summary_recovery where status = 'queued')`).Scan(&active); err != nil {
		return err
	}
	if active {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `delete from summary_recovery`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `insert into summary_recovery(kind,item_id,summary_date)
 select 'summary',id,summary_date from summaries where status='failed'
	union all select 'digest',id,summary_date from daily_digests where (status='failed' or delivery_error<>'') and delivered_at is null`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RecoveryItems 返回当前批次的稳定顺序和逐项结果，供后台执行和网页进度共用。
func (s *Store) RecoveryItems(ctx context.Context) ([]RecoveryItem, error) {
	rows, err := s.Pool.Query(ctx, `select kind,item_id,summary_date::text,started,status,error_message
 from summary_recovery order by case kind when 'summary' then 0 else 1 end,summary_date,item_id`)
	if err != nil {
		return nil, fmt.Errorf("list recovery: %w", err)
	}
	defer rows.Close()
	items := []RecoveryItem{}
	for rows.Next() {
		var item RecoveryItem
		if err := rows.Scan(&item.Kind, &item.ID, &item.Date, &item.Started, &item.Status, &item.Error); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SaveRecoveryItem 持久化执行状态，进程重启后继续处理尚未结束的任务。
func (s *Store) SaveRecoveryItem(ctx context.Context, item RecoveryItem) error {
	_, err := s.Pool.Exec(ctx, `update summary_recovery set started=$3,status=$4,error_message=$5 where kind=$1 and item_id=$2`, item.Kind, item.ID, item.Started, item.Status, item.Error)
	return err
}
