package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type FailureState struct {
	Kind        string
	Status      string
	Error       string
	RetryCount  int
	NextRetryAt *time.Time
	InDigest    bool
}

type FailureNotice struct {
	Date        string     `json:"date"`
	Message     string     `json:"message"`
	Attempts    int        `json:"attempts"`
	DeliveredAt *time.Time `json:"deliveredAt"`
	Error       string     `json:"error"`
}

// FailureDates 查找有失败记录且尚未形成通知的摘要日期。
func (s *Store) FailureDates(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `select summary_date::text from (
 select s.summary_date from summaries s join chats c on c.id=s.chat_id
 where s.status='failed' and c.summary_enabled
 union select summary_date from daily_digests where status='failed'
 ) dates where not exists(select 1 from summary_failure_notices n where n.summary_date=dates.summary_date)
 order by summary_date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	dates := []string{}
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

// FailureStates 读取启用摘要的群组及总览状态；缺失的群摘要视为尚未结束。
func (s *Store) FailureStates(ctx context.Context, date string) ([]FailureState, error) {
	rows, err := s.Pool.Query(ctx, `select 'summary',coalesce(s.status,'pending'),coalesce(s.error_message,''),coalesce(s.retry_count,0),s.next_retry_at,c.delivery_mode='bot'
 from chats c left join summaries s on s.chat_id=c.id and s.summary_date=$1::date
 where c.summary_enabled
 union all select 'digest',status,error_message,retry_count,next_retry_at,false from daily_digests where summary_date=$1::date`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FailureState{}
	for rows.Next() {
		var item FailureState
		if err := rows.Scan(&item.Kind, &item.Status, &item.Error, &item.RetryCount, &item.NextRetryAt, &item.InDigest); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// QueueFailureNotice 以摘要日期唯一约束持久化去重，同时避开正在恢复的日期。
func (s *Store) QueueFailureNotice(ctx context.Context, date, message string) error {
	_, err := s.Pool.Exec(ctx, `insert into summary_failure_notices(summary_date,message)
 select $1::date,$2 where not exists(select 1 from summary_recovery where summary_date=$1::date and status='queued')
 on conflict do nothing`, date, message)
	return err
}

// ClaimFailureNotice 用数据库租期串行领取通知，发送失败最多尝试四次。
func (s *Store) ClaimFailureNotice(ctx context.Context) (FailureNotice, error) {
	var item FailureNotice
	err := s.Pool.QueryRow(ctx, `update summary_failure_notices set attempts=attempts+1,next_attempt_at=now()+interval '5 minutes'
 where summary_date=(select summary_date from summary_failure_notices where delivered_at is null and attempts<4 and next_attempt_at<=now()
 order by summary_date limit 1 for update skip locked)
 returning summary_date::text,message,attempts`).Scan(&item.Date, &item.Message, &item.Attempts)
	return item, err
}

// FinishFailureNotice 保存发送结果，不将可能含有 Bot Token 的原始网络错误写入页面。
func (s *Store) FinishFailureNotice(ctx context.Context, date string, deliveryError string) error {
	if deliveryError != "" {
		_, err := s.Pool.Exec(ctx, `update summary_failure_notices set delivery_error=$2 where summary_date=$1::date`, date, deliveryError)
		return err
	}
	_, err := s.Pool.Exec(ctx, `update summary_failure_notices set delivered_at=now(),delivery_error='' where summary_date=$1::date`, date)
	return err
}

// FailureNotices 返回最近的通知投递记录，方便用户发现 Telegram 本身的故障。
func (s *Store) FailureNotices(ctx context.Context) ([]FailureNotice, error) {
	rows, err := s.Pool.Query(ctx, `select summary_date::text,message,attempts,delivered_at,delivery_error from summary_failure_notices order by summary_date desc limit 30`)
	if err != nil {
		return nil, fmt.Errorf("list failure notices: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByPos[FailureNotice])
}
