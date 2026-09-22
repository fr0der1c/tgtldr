package scheduler

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"golang.org/x/sync/errgroup"
)

// RunMaintenance 独立处理历史恢复和失败通知，避免阻塞每天的正常调度。
func (s *Service) RunMaintenance(ctx context.Context, origin string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.runRecovery(ctx); err != nil && ctx.Err() == nil {
			log.Printf("summary recovery: %v", err)
		}
		if err := s.retryHistoricalFailures(ctx); err != nil && ctx.Err() == nil {
			log.Printf("historical summary retry: %v", err)
		}
		if err := s.notifyFailures(ctx, origin); err != nil && ctx.Err() == nil {
			log.Printf("summary failure notification: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// runRecovery 每轮最多处理两条单群摘要，全部结束后按日期逐个重建失败总览。
func (s *Service) runRecovery(ctx context.Context) error {
	items, err := s.store.RecoveryItems(ctx)
	if err != nil {
		return err
	}
	selected := recoveryRound(items)
	var group errgroup.Group
	for _, item := range selected {
		group.Go(func() error {
			if item.Kind == "summary" {
				return s.recoverSummary(ctx, item)
			}
			return s.recoverDigest(ctx, item)
		})
	}
	return group.Wait()
}

// recoveryRound 确定性选择待运行项，确保总览不会抢在来源摘要之前执行。
func recoveryRound(items []store.RecoveryItem) []store.RecoveryItem {
	selected := []store.RecoveryItem{}
	for _, item := range items {
		if item.Status != "queued" || item.Kind != "summary" {
			continue
		}
		selected = append(selected, item)
		if len(selected) == 2 {
			return selected
		}
	}
	if len(selected) > 0 {
		return selected
	}
	for _, item := range items {
		if item.Status == "queued" {
			return []store.RecoveryItem{item}
		}
	}
	return selected
}

// recoverSummary 复用既有生成及重试流程；失败留在结果中，不取消其他任务。
func (s *Service) recoverSummary(ctx context.Context, job store.RecoveryItem) error {
	item, err := s.store.Summaries.GetByID(ctx, job.ID)
	if err != nil {
		return s.finishRecovery(ctx, job, err)
	}
	if item.Status == model.SummaryStatusSucceeded {
		return s.finishRecovery(ctx, job, nil)
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if job.Started && item.Status == model.SummaryStatusFailed {
		if item.NextRetryAt == nil || item.RetryCount >= settings.SummaryRetryLimit {
			return s.finishRecovery(ctx, job, errors.New(item.ErrorMessage))
		}
		if item.NextRetryAt.After(s.clock.Now()) {
			return nil
		}
	}
	chat, err := s.store.Chats.GetByID(ctx, item.ChatID)
	if err != nil {
		return s.finishRecovery(ctx, job, err)
	}
	retry := job.Started && item.Status == model.SummaryStatusFailed
	if retry {
		err = s.RunRetry(ctx, chat, item.SummaryDate)
	} else {
		err = s.RunNow(ctx, chat, item.SummaryDate)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	job.Started = true
	if saveErr := s.store.SaveRecoveryItem(ctx, job); saveErr != nil {
		return saveErr
	}
	if err != nil {
		if saveErr := s.store.Summaries.SetFailed(ctx, item.ChatID, item.SummaryDate, err.Error()); saveErr != nil {
			return saveErr
		}
		return s.finishRecovery(ctx, job, err)
	}
	item, err = s.store.Summaries.GetByID(ctx, job.ID)
	if err != nil {
		return err
	}
	if item.Status == model.SummaryStatusSucceeded {
		return s.finishRecovery(ctx, job, nil)
	}
	if item.Status == model.SummaryStatusFailed && (item.NextRetryAt == nil || item.RetryCount >= settings.SummaryRetryLimit) {
		return s.finishRecovery(ctx, job, errors.New(item.ErrorMessage))
	}
	return nil
}

// recoverDigest 仅重建批次中的失败总览，保留已经发送的成功总览。
func (s *Service) recoverDigest(ctx context.Context, job store.RecoveryItem) error {
	item, err := s.dailyDigests.Get(ctx, job.ID)
	if err != nil {
		return s.finishRecovery(ctx, job, err)
	}
	if item.Status == model.SummaryStatusSucceeded {
		if item.DeliveredAt == nil && item.DeliverySkippedReason == "" {
			err = s.dailyDigests.RetryDelivery(ctx, item.ID)
			if err != nil {
				return s.finishRecovery(ctx, job, err)
			}
			item, err = s.dailyDigests.Get(ctx, job.ID)
			if err != nil {
				return err
			}
			if item.DeliveredAt == nil {
				return nil
			}
		}
		return s.finishRecovery(ctx, job, err)
	}
	if item.Status == model.SummaryStatusRunning || item.Status == model.SummaryStatusPending {
		return nil
	}
	if job.Started {
		settings, err := s.store.Settings.Get(ctx)
		if err != nil {
			return err
		}
		if item.NextRetryAt != nil && item.RetryCount < settings.SummaryRetryLimit {
			return s.dailyDigests.ContinueExisting(ctx, settings, item)
		}
		return s.finishRecovery(ctx, job, errors.New(item.ErrorMessage))
	}
	err = s.dailyDigests.Rerun(ctx, job.ID)
	if err != nil {
		return s.finishRecovery(ctx, job, err)
	}
	job.Started = true
	return s.store.SaveRecoveryItem(ctx, job)
}

// finishRecovery 记录单项最终结果，失败详情在网页中可查。
func (s *Service) finishRecovery(ctx context.Context, job store.RecoveryItem, err error) error {
	job.Status = "succeeded"
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
	}
	return s.store.SaveRecoveryItem(ctx, job)
}
