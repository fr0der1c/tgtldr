package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type HistoricalRetry struct {
	Kind string
	ID   int64
}

// RecoverInterruptedSummaries 将进程退出时中断的摘要交回有限重试流程。
func (s *Store) RecoverInterruptedSummaries(ctx context.Context, message string) error {
	_, err := s.Pool.Exec(ctx, `update summaries set status='failed',error_message=$1,next_retry_at=now(),updated_at=now() where status in ('pending','running')`, message)
	return err
}

// HistoricalRetries 读取普通调度日期之外的到期重试，排除已经进入批量恢复队列的任务。
func (s *Store) HistoricalRetries(ctx context.Context, now time.Time, scheduledDate string, retryLimit int, botReady bool) ([]HistoricalRetry, error) {
	rows, err := s.Pool.Query(ctx, `select kind,id from (
 select 'summary' as kind,s.id,s.next_retry_at from summaries s join chats c on c.id=s.chat_id
 where s.status='failed' and c.summary_enabled and s.summary_date<>$2::date and s.next_retry_at<=$1 and s.retry_count<$3
 union all select 'digest',id,next_retry_at from daily_digests
 where status='failed' and summary_date<>$2::date and next_retry_at<=$1 and retry_count<$3 and $4
 and not exists(select 1 from daily_digests active where active.status in ('pending','running'))
 ) tasks where not exists(select 1 from summary_recovery q where q.kind=tasks.kind and q.item_id=tasks.id and q.status='queued')
 order by next_retry_at,kind,id limit 2`, now, scheduledDate, retryLimit, botReady)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByPos[HistoricalRetry])
}
