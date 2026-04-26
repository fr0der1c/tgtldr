package store

import (
	"context"
	"fmt"

	"github.com/frederic/tgtldr/app/internal/model"
)

func (r *SummaryRepository) Stats(ctx context.Context) (model.SummaryStats, error) {
	var stats model.SummaryStats
	err := r.pool.QueryRow(ctx, `
		select
			count(*) as total,
			count(*) filter (where status = 'succeeded') as success_count,
			count(*) filter (where status in ('pending', 'running')) as processing_count,
			count(*) filter (where status = 'failed') as failed_count
		from summaries
	`).Scan(
		&stats.Total,
		&stats.SuccessCount,
		&stats.ProcessingCount,
		&stats.FailedCount,
	)
	if err != nil {
		return model.SummaryStats{}, fmt.Errorf("query summary stats: %w", err)
	}
	return stats, nil
}
