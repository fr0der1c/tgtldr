package scheduler

import (
	"context"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"golang.org/x/sync/errgroup"
)

// RecoverInterruptedSummaries 在调度器启动前恢复被重启打断的任务，并保留已有重试次数。
func (s *Service) RecoverInterruptedSummaries(ctx context.Context) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	message := "摘要因应用重启而中断，将按配置继续重试。"
	if settings.Language == model.LanguageEN {
		message = "Summary was interrupted by an application restart and will follow the configured retry policy."
	}
	return s.store.RecoverInterruptedSummaries(ctx, message)
}

// retryHistoricalFailures 继续执行跨日及手动历史补跑的有限重试，确保失败通知能到达终态。
func (s *Service) retryHistoricalFailures(ctx context.Context) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	botReady := settings.BotEnabled && strings.TrimSpace(settings.BotToken) != "" && strings.TrimSpace(settings.BotTargetChatID) != ""
	items, err := s.store.HistoricalRetries(ctx, now, targetDate(now, settings.DefaultTimezone), settings.SummaryRetryLimit, botReady)
	if err != nil {
		return err
	}
	var group errgroup.Group
	for _, item := range items {
		group.Go(func() error { return s.retryHistoricalItem(ctx, settings, item) })
	}
	return group.Wait()
}

// retryHistoricalItem 复用原有重试计数与错误落库规则，单项出错不取消其他任务。
func (s *Service) retryHistoricalItem(ctx context.Context, settings model.AppSettings, item store.HistoricalRetry) error {
	if item.Kind == "digest" {
		digest, err := s.dailyDigests.Get(ctx, item.ID)
		if err != nil {
			return err
		}
		return s.dailyDigests.ContinueExisting(ctx, settings, digest)
	}
	summary, err := s.store.Summaries.GetByID(ctx, item.ID)
	if err != nil {
		return err
	}
	chat, err := s.store.Chats.GetByID(ctx, summary.ChatID)
	if err != nil {
		return err
	}
	if err = s.RunRetry(ctx, chat, summary.SummaryDate); err != nil && ctx.Err() == nil {
		if saveErr := s.store.Summaries.SetFailed(ctx, chat.ID, summary.SummaryDate, err.Error()); saveErr != nil {
			return saveErr
		}
	}
	return err
}
