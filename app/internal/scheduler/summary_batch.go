package scheduler

import (
	"context"
	"errors"
	"sync"

	"github.com/fr0der1c/tgtldr/app/internal/catchup"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

const (
	summaryBatchParallelism = 2
	// SummaryBatchMaxItems 限制单批的群组日期数量，避免一次占满模型额度。
	SummaryBatchMaxItems = 200
)

var (
	ErrSummaryBatchChats    = errors.New("some selected chats are unavailable for batch generation")
	ErrSummaryBatchTooLarge = errors.New("summary batch has too many items")
)

// SummaryBatchPlan 说明本次提交排队和跳过的数量。
type SummaryBatchPlan struct {
	Queued  int `json:"queued"`
	Skipped int `json:"skipped"`
}

// QueueSummaryBatch 校验所选群组与日期范围，并把需要生成的群组日期按日期顺序排队。
// overwrite 为 false 时跳过已经生成成功的摘要。
func (s *Service) QueueSummaryBatch(ctx context.Context, chatIDs []int64, fromDate, toDate string, overwrite bool) (SummaryBatchPlan, error) {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return SummaryBatchPlan{}, err
	}
	if err = catchup.ValidateDateRange(fromDate, toDate, settings.DefaultTimezone, s.clock.Now()); err != nil {
		return SummaryBatchPlan{}, err
	}
	chats, err := s.summaryBatchChats(ctx, chatIDs)
	if err != nil {
		return SummaryBatchPlan{}, err
	}
	dates := datesInRange(fromDate, toDate, settings.DefaultTimezone)
	if len(chats)*len(dates) > SummaryBatchMaxItems {
		return SummaryBatchPlan{}, ErrSummaryBatchTooLarge
	}
	plan := SummaryBatchPlan{}
	items := make([]store.SummaryBatchItem, 0, len(chats)*len(dates))
	for _, date := range dates {
		for _, chat := range chats {
			if !overwrite {
				item, found, err := s.lookupSummary(ctx, chat.ID, date)
				if err != nil {
					return SummaryBatchPlan{}, err
				}
				if found && item.Status == model.SummaryStatusSucceeded {
					plan.Skipped++
					continue
				}
			}
			items = append(items, store.SummaryBatchItem{ChatID: chat.ID, Date: date, Overwrite: overwrite})
		}
	}
	if len(items) == 0 {
		return plan, s.store.EnsureNoActiveSummaryBatch(ctx)
	}
	if err = s.store.QueueSummaryBatch(ctx, items); err != nil {
		return SummaryBatchPlan{}, err
	}
	plan.Queued = len(items)
	return plan, nil
}

// summaryBatchChats 去重读取所选群组，只接受已开启 AI 摘要的群组。
func (s *Service) summaryBatchChats(ctx context.Context, chatIDs []int64) ([]model.Chat, error) {
	seen := map[int64]bool{}
	chats := []model.Chat{}
	for _, id := range chatIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		chat, err := s.store.Chats.GetByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !chat.SummaryEnabled {
			return nil, ErrSummaryBatchChats
		}
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	if len(chats) == 0 {
		return nil, ErrSummaryBatchChats
	}
	return chats, nil
}

// runSummaryBatch 每轮最多开始生成两条批量摘要，生成与手动“立即生成”相同。每项都在持有
// 生成任务时处理：正在其他流程中生成的项留到下一轮；生成前即可落定的跳过项和已开始的项不占用名额。
func (s *Service) runSummaryBatch(ctx context.Context) error {
	items, err := s.store.SummaryBatchItems(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	record := func(err error) {
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}
	started := 0
	for _, item := range items {
		if item.Status != "queued" || !item.Started && started == summaryBatchParallelism {
			continue
		}
		key := summaryTaskKey(item.ChatID, item.Date)
		if !s.beginTask(key) {
			continue
		}
		if item.Started {
			wg.Go(func() {
				defer s.finishTask(key)
				record(s.checkSummaryBatchItem(ctx, settings, item))
			})
			continue
		}
		chat, ready, err := s.prepareSummaryBatchItem(ctx, item)
		if err != nil || !ready {
			s.finishTask(key)
			record(err)
			continue
		}
		started++
		wg.Go(func() {
			defer s.finishTask(key)
			record(s.startSummaryBatchItem(ctx, settings, chat, item))
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// prepareSummaryBatchItem 在生成前复查跳过条件；ready 为 false 表示该项已经落定。
func (s *Service) prepareSummaryBatchItem(ctx context.Context, job store.SummaryBatchItem) (model.Chat, bool, error) {
	chat, err := s.store.Chats.GetByID(ctx, job.ChatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return chat, false, s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "failed", "chat no longer exists")
	}
	if err != nil {
		return chat, false, err
	}
	if !chat.SummaryEnabled {
		return chat, false, s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "skipped", "AI summaries are disabled for this chat")
	}
	if !job.Overwrite {
		item, found, err := s.lookupSummary(ctx, job.ChatID, job.Date)
		if err != nil {
			return chat, false, err
		}
		if found && item.Status == model.SummaryStatusSucceeded {
			return chat, false, s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "skipped", "")
		}
	}
	return chat, true, nil
}

// startSummaryBatchItem 记录开始时间并调用 runNow，随后按摘要状态落定或继续等待。
// 出错时只把仍在生成中的摘要标记失败，不改动已经保存的结果；本次未写入摘要时直接记为失败，避免反复重跑。
func (s *Service) startSummaryBatchItem(ctx context.Context, settings model.AppSettings, chat model.Chat, job store.SummaryBatchItem) error {
	startedAt, err := s.store.StartSummaryBatchItem(ctx, job.ChatID, job.Date)
	if err != nil {
		return err
	}
	job.Started, job.StartedAt = true, startedAt
	err = s.runNow(ctx, chat, job.Date)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil {
		return s.checkSummaryBatchItem(ctx, settings, job)
	}
	item, found, lookupErr := s.lookupSummary(ctx, job.ChatID, job.Date)
	if lookupErr != nil {
		return lookupErr
	}
	if !found || item.UpdatedAt.Before(job.StartedAt) {
		return s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "failed", err.Error())
	}
	if item.Status == model.SummaryStatusPending || item.Status == model.SummaryStatusRunning {
		if saveErr := s.store.Summaries.SetFailed(ctx, chat.ID, job.Date, err.Error()); saveErr != nil {
			return saveErr
		}
	}
	return s.checkSummaryBatchItem(ctx, settings, job)
}

// checkSummaryBatchItem 在持有生成任务时确认已开始项的结果；等待自动重试时保持排队。
// 没有摘要、摘要停在生成中或开始后未被更新，说明进程曾中断，退回未开始；
// 等待重试期间关闭 AI 总结的群组不会再重试，记为跳过。
func (s *Service) checkSummaryBatchItem(ctx context.Context, settings model.AppSettings, job store.SummaryBatchItem) error {
	item, found, err := s.lookupSummary(ctx, job.ChatID, job.Date)
	if err != nil {
		return err
	}
	switch {
	case !found, item.Status == model.SummaryStatusPending, item.Status == model.SummaryStatusRunning,
		item.UpdatedAt.Before(job.StartedAt):
		return s.store.ResetSummaryBatchItem(ctx, job.ChatID, job.Date)
	case item.Status == model.SummaryStatusSucceeded:
		return s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "succeeded", "")
	case item.NextRetryAt == nil || item.RetryCount >= settings.SummaryRetryLimit:
		return s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "failed", item.ErrorMessage)
	}
	chat, err := s.store.Chats.GetByID(ctx, job.ChatID)
	if err != nil {
		return err
	}
	if !chat.SummaryEnabled {
		return s.store.FinishSummaryBatchItem(ctx, job.ChatID, job.Date, "skipped", "AI summaries were disabled before the retry")
	}
	return nil
}
