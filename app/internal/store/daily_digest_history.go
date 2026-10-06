package store

import (
	"context"
	"fmt"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// ListHistory 返回目标日期前指定自然日内的成功总览，按日期由近到远排列。
func (r *DailyDigestRepository) ListHistory(ctx context.Context, summaryDate string, days int) ([]model.DailyDigest, error) {
	rows, err := r.pool.Query(ctx, `
		select summary_date::text, content
		from daily_digests
		where summary_date >= $1::date - $2::integer
		  and summary_date < $1::date
		  and status = 'succeeded' and btrim(content) <> ''
		  and execution_mode <> 'no_content'
		order by summary_date desc
	`, summaryDate, days)
	if err != nil {
		return nil, fmt.Errorf("list daily digest history before %s: %w", summaryDate, err)
	}
	defer rows.Close()

	items := make([]model.DailyDigest, 0, days)
	for rows.Next() {
		var item model.DailyDigest
		if err := rows.Scan(&item.SummaryDate, &item.Content); err != nil {
			return nil, fmt.Errorf("scan daily digest history before %s: %w", summaryDate, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily digest history before %s: %w", summaryDate, err)
	}
	return items, nil
}
