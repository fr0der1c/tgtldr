package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/dailydigest"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	store        *store.Store
	clock        clock.Clock
	summaries    *summary.Service
	botService   *bot.Service
	dailyDigests *dailydigest.Service
	mu           sync.Mutex
	inflight     map[string]struct{}
}

type scheduledAction int

const (
	scheduledActionSkip scheduledAction = iota
	scheduledActionGenerate
	scheduledActionRetry
	scheduledActionDeliver
)

func NewService(
	st *store.Store,
	c clock.Clock,
	summaries *summary.Service,
	botService *bot.Service,
	dailyDigests *dailydigest.Service,
) *Service {
	return &Service{
		store:        st,
		clock:        c,
		summaries:    summaries,
		botService:   botService,
		dailyDigests: dailyDigests,
		inflight:     make(map[string]struct{}),
	}
}

func (s *Service) ContextPreview(ctx context.Context, item model.Summary) (model.SummaryContextPreview, error) {
	return s.summaries.BuildContextPreview(ctx, item)
}

func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	if err := s.runOnce(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.runOnce(ctx); err != nil {
				continue
			}
		}
	}
}

func (s *Service) RunNow(ctx context.Context, chat model.Chat, date string) error {
	date, period, err := s.summaryTarget(ctx, chat, date)
	if err != nil {
		return err
	}
	key := summaryTaskKey(chat.ID, date)
	if !s.beginTask(key) {
		return nil
	}
	defer s.finishTask(key)
	return s.runNow(ctx, chat, date, period)
}

func (s *Service) RunRetry(ctx context.Context, chat model.Chat, date string) error {
	key := summaryTaskKey(chat.ID, date)
	if !s.beginTask(key) {
		return nil
	}
	defer s.finishTask(key)

	if err := s.store.Summaries.SetRetryRunning(ctx, chat.ID, date); err != nil {
		return err
	}
	return s.executeSummary(ctx, chat, date)
}

func (s *Service) RunNowAsync(ctx context.Context, chat model.Chat, date string) (bool, error) {
	date, period, err := s.summaryTarget(ctx, chat, date)
	if err != nil {
		return false, err
	}
	key := summaryTaskKey(chat.ID, date)
	if !s.beginTask(key) {
		return false, nil
	}

	deliveryMode, err := s.deliveryModeForPeriod(ctx, date, period)
	if err != nil {
		s.finishTask(key)
		return false, err
	}
	if err := s.store.Summaries.UpsertPending(ctx, chat.ID, date, deliveryMode, period); err != nil {
		s.finishTask(key)
		return false, err
	}
	if err := s.store.Summaries.SetRunning(ctx, chat.ID, date); err != nil {
		s.finishTask(key)
		return false, err
	}

	go func() {
		defer s.finishTask(key)
		runCtx := context.Background()
		if err := s.executeSummary(runCtx, chat, date); err != nil {
			_ = s.store.Summaries.SetFailed(context.Background(), chat.ID, date, err.Error())
		}
	}()
	return true, nil
}

func (s *Service) RetryDelivery(ctx context.Context, summaryID int64) error {
	item, err := s.store.Summaries.GetByID(ctx, summaryID)
	if err != nil {
		return err
	}
	if item.Status != model.SummaryStatusSucceeded {
		return fmt.Errorf("只有生成成功的摘要才能重试发送")
	}

	chat, err := s.store.Chats.GetByID(ctx, item.ChatID)
	if err != nil {
		return err
	}
	if chat.DeliveryMode != model.DeliveryModeBot {
		return fmt.Errorf("当前群组设置为不发送")
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if item.DailyDigestID > 0 || item.BotSummaryDeliveryMode == model.BotSummaryDeliveryModeDailyDigest {
		return fmt.Errorf("每日总览模式下不能单独发送群组摘要")
	}
	if !periodSummaryDeliverable(item, settings.DefaultTimezone) {
		// 与自动推送一致：周期结束前生成的部分内容不发送，避免周期结束后重新生成时重复推送。
		if settings.Language == model.LanguageEN {
			return fmt.Errorf("This period has not ended yet. The summary will be regenerated and sent after the period ends.")
		}
		return fmt.Errorf("周期尚未结束，周期结束后会重新生成并发送")
	}

	key := summaryTaskKey(chat.ID, item.SummaryDate)
	if !s.beginTask(key) {
		return nil
	}
	defer s.finishTask(key)

	if err := s.deliverSummary(ctx, settings, chat, item); err != nil {
		_ = s.store.Summaries.MarkDeliveryFailed(ctx, item.ChatID, item.SummaryDate, err.Error())
		return err
	}
	return s.store.Summaries.MarkDelivered(ctx, item.ChatID, item.SummaryDate, s.clock.Now())
}

func (s *Service) RepairEmptySummariesInRange(ctx context.Context, chat model.Chat, fromDate, toDate string) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}

	timezone := settings.DefaultTimezone
	seen := map[string]bool{}
	for _, day := range datesInRange(fromDate, toDate, timezone) {
		// 每周和每月摘要记在周期第一天，按所在周期去重检查。
		date, _, err := s.summaryTarget(ctx, chat, day)
		if err != nil {
			return err
		}
		if seen[date] {
			continue
		}
		seen[date] = true
		item, found, err := s.lookupSummary(ctx, chat.ID, date)
		if err != nil {
			return err
		}
		if !found || !isRepairableEmptySummary(item) {
			continue
		}

		start, _, err := summaryDayRange(date, timezone)
		if err != nil {
			return err
		}
		_, end, err := summaryDayRange(item.PeriodEnd, timezone)
		if err != nil {
			return err
		}
		messageCount, err := s.store.Messages.CountForRange(ctx, chat.ID, start, end)
		if err != nil {
			return err
		}
		if messageCount == 0 {
			continue
		}
		if _, err := s.RunNowAsync(ctx, chat, date); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runNow(ctx context.Context, chat model.Chat, date string, period model.SummaryPeriod) error {
	deliveryMode, err := s.deliveryModeForPeriod(ctx, date, period)
	if err != nil {
		return err
	}
	if err := s.store.Summaries.UpsertPending(ctx, chat.ID, date, deliveryMode, period); err != nil {
		return err
	}
	if err := s.store.Summaries.SetRunning(ctx, chat.ID, date); err != nil {
		return err
	}
	return s.executeSummary(ctx, chat, date)
}

func (s *Service) executeSummary(ctx context.Context, chat model.Chat, date string) error {
	result, err := s.summaries.RunDailySummary(ctx, chat, date)
	if err != nil {
		return err
	}
	if err := s.store.Summaries.SaveResult(ctx, result); err != nil {
		return err
	}
	if result.Status != model.SummaryStatusSucceeded {
		if result.RetryableError {
			return s.scheduleNextRetry(ctx, result)
		}
		return nil
	}
	storedResult, err := s.store.Summaries.GetByChatAndDate(ctx, result.ChatID, result.SummaryDate)
	if err != nil {
		return err
	}
	s.tryDeliverSummary(ctx, chat, storedResult)
	return nil
}

// deliveryModeForPeriod 解析新建摘要应固化的 Bot 包装方式。
func (s *Service) deliveryModeForPeriod(ctx context.Context, date string, period model.SummaryPeriod) (model.BotSummaryDeliveryMode, error) {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return "", err
	}
	return resolveDeliveryMode(settings, date, period), nil
}

// resolveDeliveryMode 沿用日期级 Bot 包装方式；每周和每月摘要不进入每日总览，始终单独推送。
func resolveDeliveryMode(settings model.AppSettings, date string, period model.SummaryPeriod) model.BotSummaryDeliveryMode {
	if model.NormalizeSummaryPeriod(period) != model.SummaryPeriodDay {
		return model.BotSummaryDeliveryModePerChat
	}
	return model.ResolveBotSummaryDeliveryMode(settings, date)
}

// summaryTarget 解析手动生成或补跑的目标：已有摘要沿用自己的周期；
// 否则按群组在该日期的频率，生成它所在的整个周期，摘要记在周期第一天。
func (s *Service) summaryTarget(ctx context.Context, chat model.Chat, date string) (string, model.SummaryPeriod, error) {
	if item, found, err := s.lookupSummary(ctx, chat.ID, date); err != nil || found {
		return date, item.Period, err
	}
	start, period, err := chat.SummaryPeriodStart(date)
	if err != nil {
		return "", model.SummaryPeriodDay, fmt.Errorf("parse summary date %s: %w", date, err)
	}
	if start == date {
		return start, period, nil
	}
	if item, found, err := s.lookupSummary(ctx, chat.ID, start); err != nil || found {
		return start, item.Period, err
	}
	return start, period, nil
}

func (s *Service) runOnce(ctx context.Context) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}

	chats, err := s.store.Chats.ListSummaryEnabled(ctx)
	if err != nil {
		return err
	}

	now := s.clock.Now()
	timezone := settings.DefaultTimezone
	yesterday := targetDate(now, timezone)
	dailyChats := make([]model.Chat, 0, len(chats))
	group, groupCtx := errgroup.WithContext(ctx)
	for _, chat := range chats {
		chat := chat
		if chat.SummaryPeriodFor(yesterday) == model.SummaryPeriodDay {
			dailyChats = append(dailyChats, chat)
		}
		if !isDue(now, chat, timezone) {
			continue
		}
		// 每周和每月摘要在周期结束后的首个摘要时间生成；停机错过时只补最近一个已结束的周期。
		date, period, err := chat.LatestEndedPeriod(yesterday)
		if err != nil {
			continue
		}
		group.Go(func() error {
			deliveryMode := resolveDeliveryMode(settings, date, period)
			if err := s.store.Summaries.SetBotSummaryDeliveryMode(groupCtx, chat.ID, date, deliveryMode); err != nil {
				return err
			}
			item, found, err := s.lookupSummary(groupCtx, chat.ID, date)
			if err != nil {
				return err
			}
			if found && (item.Period != period || item.PeriodEnd > yesterday) {
				// 切换频率前留下的其他周期摘要不由定时任务处理，避免反复重新生成。
				return nil
			}

			switch decideScheduledAction(chat, item, found, timezone, settings, now) {
			case scheduledActionSkip:
				return nil
			case scheduledActionRetry:
				if date != yesterday {
					// 不是昨天的周期由历史重试流程负责，避免两处同时重试。
					return nil
				}
				return s.RunRetry(groupCtx, chat, date)
			case scheduledActionDeliver:
				s.deliverExistingSummary(groupCtx, chat, item)
				return nil
			default:
				key := summaryTaskKey(chat.ID, date)
				if !s.beginTask(key) {
					return nil
				}
				defer s.finishTask(key)
				return s.runNow(groupCtx, chat, date, period)
			}
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return s.dailyDigests.RunIfReady(ctx, settings, dailyChats, yesterday)
}

func (s *Service) scheduleNextRetry(ctx context.Context, result model.Summary) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	item, err := s.store.Summaries.GetByChatAndDate(ctx, result.ChatID, result.SummaryDate)
	if err != nil {
		return err
	}
	nextRetryAt, ok := nextSummaryRetryAt(settings, item.RetryCount, s.clock.Now())
	if !ok {
		return nil
	}
	return s.store.Summaries.ScheduleRetry(ctx, result.ChatID, result.SummaryDate, nextRetryAt)
}

func (s *Service) deliverExistingSummary(ctx context.Context, chat model.Chat, result model.Summary) {
	key := summaryTaskKey(chat.ID, result.SummaryDate)
	if !s.beginTask(key) {
		return
	}
	defer s.finishTask(key)
	s.tryDeliverSummary(ctx, chat, result)
}

func (s *Service) tryDeliverSummary(ctx context.Context, chat model.Chat, result model.Summary) {
	if chat.DeliveryMode != model.DeliveryModeBot {
		return
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		_ = s.store.Summaries.MarkDeliveryFailed(ctx, result.ChatID, result.SummaryDate, err.Error())
		return
	}
	if result.BotSummaryDeliveryMode == model.BotSummaryDeliveryModeDailyDigest {
		return
	}
	if !periodSummaryDeliverable(result, settings.DefaultTimezone) {
		return
	}

	if err := s.deliverSummary(ctx, settings, chat, result); err != nil {
		_ = s.store.Summaries.MarkDeliveryFailed(ctx, result.ChatID, result.SummaryDate, err.Error())
		return
	}
	_ = s.store.Summaries.MarkDelivered(ctx, result.ChatID, result.SummaryDate, s.clock.Now())
}

func (s *Service) deliverSummary(ctx context.Context, settings model.AppSettings, chat model.Chat, result model.Summary) error {
	if !settings.BotEnabled {
		return fmt.Errorf("bot delivery is disabled")
	}
	if strings.TrimSpace(settings.BotToken) == "" || strings.TrimSpace(settings.BotTargetChatID) == "" {
		return fmt.Errorf("bot delivery target is not configured")
	}

	message := buildBotDeliveryMessage(chat, result, settings.Language)
	return s.botService.SendMessageWithLanguage(ctx, settings.BotToken, settings.BotTargetChatID, message, settings.Language)
}

func buildBotDeliveryMessage(chat model.Chat, result model.Summary, language model.Language) string {
	header := fmt.Sprintf("**%s · %s**", chat.Title, summaryDateLabel(result, language))
	content := strings.TrimSpace(result.Content)
	if content == "" {
		return header
	}
	return header + "\n\n" + content
}

// summaryDateLabel 显示摘要覆盖的日期；每周和每月摘要显示完整范围。
func summaryDateLabel(item model.Summary, language model.Language) string {
	if item.PeriodEnd == "" || item.PeriodEnd == item.SummaryDate {
		return item.SummaryDate
	}
	if language == model.LanguageEN {
		return item.SummaryDate + " to " + item.PeriodEnd
	}
	return item.SummaryDate + " 至 " + item.PeriodEnd
}

func (s *Service) lookupSummary(ctx context.Context, chatID int64, date string) (model.Summary, bool, error) {
	item, err := s.store.Summaries.GetByChatAndDate(ctx, chatID, date)
	if err == nil {
		return item, true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Summary{}, false, nil
	}
	return model.Summary{}, false, err
}

func decideScheduledAction(
	chat model.Chat,
	item model.Summary,
	found bool,
	timezone string,
	settings model.AppSettings,
	now time.Time,
) scheduledAction {
	if !found {
		return scheduledActionGenerate
	}
	if item.Status == model.SummaryStatusFailed {
		if shouldRetrySummary(settings, item, now) {
			return scheduledActionRetry
		}
		return scheduledActionSkip
	}
	if item.Status != model.SummaryStatusSucceeded {
		return scheduledActionGenerate
	}
	if model.NormalizeSummaryPeriod(item.Period) != model.SummaryPeriodDay && !summaryReadyForDelivery(item, timezone) {
		// 周期未结束时手动生成的每周或每月摘要只是部分内容，周期结束后重新生成。
		return scheduledActionGenerate
	}
	if chat.DeliveryMode == model.DeliveryModeBot &&
		item.BotSummaryDeliveryMode == model.BotSummaryDeliveryModeDailyDigest {
		if !summaryReadyForDelivery(item, timezone) {
			return scheduledActionGenerate
		}
		return scheduledActionSkip
	}
	if chat.DeliveryMode != model.DeliveryModeBot {
		return scheduledActionSkip
	}
	if item.DeliveredAt != nil {
		return scheduledActionSkip
	}
	if !summaryReadyForDelivery(item, timezone) {
		return scheduledActionGenerate
	}
	return scheduledActionDeliver
}

// periodSummaryDeliverable 判断摘要能否推送：周期结束前生成的每周或每月摘要只是部分内容，
// 等周期结束后重新生成时再推送；每日摘要不受影响。
func periodSummaryDeliverable(item model.Summary, timezone string) bool {
	return model.NormalizeSummaryPeriod(item.Period) == model.SummaryPeriodDay || summaryReadyForDelivery(item, timezone)
}

func summaryReadyForDelivery(item model.Summary, timezone string) bool {
	location, err := loadSummaryLocation(timezone)
	if err != nil {
		return false
	}
	lastDay := item.PeriodEnd
	if lastDay == "" {
		lastDay = item.SummaryDate
	}
	periodEnd, err := time.ParseInLocation("2006-01-02", lastDay, location)
	if err != nil {
		return false
	}
	windowEnd := periodEnd.AddDate(0, 0, 1)
	return !item.GeneratedAt.Before(windowEnd)
}

func isRepairableEmptySummary(item model.Summary) bool {
	return item.Status == model.SummaryStatusSucceeded &&
		item.SourceMessageCount == 0 &&
		item.ChunkCount == 0
}

func isDue(now time.Time, chat model.Chat, timezone string) bool {
	location, err := loadSummaryLocation(timezone)
	if err != nil {
		return false
	}
	localNow := now.In(location)
	scheduled, err := time.ParseInLocation("15:04", chat.SummaryTimeLocal, location)
	if err != nil {
		return false
	}

	scheduledTime := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		scheduled.Hour(),
		scheduled.Minute(),
		0,
		0,
		location,
	)
	return !localNow.Before(scheduledTime)
}

// ChangeSummaryFrequency 按默认时区中的今天切换群组的摘要频率，调用方负责保存。
func (s *Service) ChangeSummaryFrequency(ctx context.Context, chat *model.Chat, frequency model.SummaryPeriod) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	location, err := loadSummaryLocation(settings.DefaultTimezone)
	if err != nil {
		return err
	}
	return chat.ChangeSummaryFrequency(frequency, s.clock.Now().In(location).Format("2006-01-02"))
}

func targetDate(now time.Time, timezone string) string {
	location, err := loadSummaryLocation(timezone)
	if err != nil {
		location = time.Local
	}
	localNow := now.In(location)
	return localNow.AddDate(0, 0, -1).Format("2006-01-02")
}

func datesInRange(fromDate, toDate, timezone string) []string {
	start, _, err := summaryDayRange(fromDate, timezone)
	if err != nil {
		return nil
	}
	endStart, _, err := summaryDayRange(toDate, timezone)
	if err != nil {
		return nil
	}
	location, err := loadSummaryLocation(timezone)
	if err != nil {
		return nil
	}

	dates := make([]string, 0)
	for current := start.In(location); !current.After(endStart.In(location)); current = current.AddDate(0, 0, 1) {
		dates = append(dates, current.Format("2006-01-02"))
	}
	return dates
}

func summaryDayRange(date string, timezone string) (time.Time, time.Time, error) {
	location, err := loadSummaryLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse summary date %s: %w", date, err)
	}
	end := start.AddDate(0, 0, 1)
	return start.UTC(), end.UTC(), nil
}

func loadSummaryLocation(timezone string) (*time.Location, error) {
	if strings.TrimSpace(timezone) == "" {
		return time.Local, nil
	}
	return time.LoadLocation(timezone)
}

func summaryTaskKey(chatID int64, date string) string {
	return fmt.Sprintf("%d:%s", chatID, date)
}

func (s *Service) beginTask(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.inflight[key]; exists {
		return false
	}
	s.inflight[key] = struct{}{}
	return true
}

func (s *Service) finishTask(key string) {
	s.mu.Lock()
	delete(s.inflight, key)
	s.mu.Unlock()
}
