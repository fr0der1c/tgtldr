package dailydigest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

var (
	ErrBotUnavailable  = errors.New("telegram Bot is not configured")
	ErrSourcesNotReady = errors.New("daily digest sources are not ready")
)

type participant struct {
	chatID    int64
	chatTitle string
}

type Service struct {
	root          context.Context
	store         *store.Store
	clock         clock.Clock
	botService    *bot.Service
	openAITimeout time.Duration
	mu            sync.Mutex
	inflight      map[string]struct{}
}

// NewService 使用进程级上下文承载不受 HTTP 请求生命周期限制的后台任务。
func NewService(
	root context.Context,
	st *store.Store,
	c clock.Clock,
	botService *bot.Service,
	openAITimeout time.Duration,
) *Service {
	return &Service{
		root: root, store: st, clock: c, botService: botService,
		openAITimeout: openAITimeout, inflight: make(map[string]struct{}),
	}
}

// RecoverInterrupted 将上次进程遗留的运行中任务交回重试流程。
func (s *Service) RecoverInterrupted(ctx context.Context) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	message := "每日总览因应用重启而中断，可按重试规则继续处理。"
	if settings.Language == model.LanguageEN {
		message = "Daily Digest was interrupted by an application restart and can continue under the retry policy."
	}
	return s.store.DailyDigests.RecoverInterrupted(ctx, message)
}

func (s *Service) Get(ctx context.Context, id int64) (model.DailyDigest, error) {
	return s.store.DailyDigests.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, page, pageSize int) (model.DailyDigestListResponse, error) {
	return s.store.DailyDigests.List(ctx, page, pageSize)
}

// RunIfReady 在参与群组全部到达终态后创建或继续当天的每日总览。
func (s *Service) RunIfReady(ctx context.Context, settings model.AppSettings, chats []model.Chat, summaryDate string) error {
	if model.ResolveBotSummaryDeliveryMode(settings, summaryDate) != model.BotSummaryDeliveryModeDailyDigest {
		return nil
	}
	if !botReady(settings) {
		return nil
	}
	if item, err := s.store.DailyDigests.GetByDate(ctx, summaryDate); err == nil {
		return s.continueExisting(ctx, settings, item)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	selectedChats := participantChats(chats)
	if len(selectedChats) == 0 || !allParticipantsDue(s.clock.Now(), selectedChats, settings.DefaultTimezone) {
		return nil
	}
	sources, ready, err := s.collectSources(ctx, settings, summaryDate, participantsFromChats(selectedChats))
	if errors.Is(err, ErrSourcesNotReady) {
		return nil
	}
	if err != nil || !ready {
		return err
	}
	item, created, err := s.store.DailyDigests.Create(ctx, summaryDate, sources)
	if err != nil {
		return err
	}
	if !created {
		return s.continueExisting(ctx, settings, item)
	}
	return s.startGeneration(ctx, item.ID, false, true)
}

// Rerun 按最新参与配置重建并发送；来源未就绪时保留已有总览。
func (s *Service) Rerun(ctx context.Context, id int64) error {
	key := fmt.Sprintf("rerun:%d", id)
	if !s.begin(key) {
		return ErrSourcesNotReady
	}
	defer s.finish(key)
	item, err := s.store.DailyDigests.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item.Status == model.SummaryStatusPending || item.Status == model.SummaryStatusRunning {
		return ErrSourcesNotReady
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if !botReady(settings) {
		return ErrBotUnavailable
	}
	chats, err := s.store.Chats.ListSummaryEnabled(ctx)
	if err != nil {
		return err
	}
	participants := participantsFromChats(participantChats(chats))
	if len(participants) == 0 {
		if settings.Language == model.LanguageEN {
			return fmt.Errorf("No chats are participating in Daily Digest")
		}
		return fmt.Errorf("没有参与每日总览的群组")
	}
	sources, ready, err := s.collectSources(ctx, settings, item.SummaryDate, participants)
	if err != nil {
		return err
	}
	if !ready {
		return ErrSourcesNotReady
	}
	if err := s.store.DailyDigests.PrepareRegeneration(ctx, id, sources); err != nil {
		return err
	}
	return s.startGeneration(ctx, id, false, true)
}

// RetryDelivery 只重新发送已经生成的正文，不重新调用模型。
func (s *Service) RetryDelivery(ctx context.Context, id int64) error {
	item, err := s.store.DailyDigests.GetByID(ctx, id)
	if err != nil {
		return err
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if item.Status != model.SummaryStatusSucceeded || item.DeliverySkippedReason != "" {
		if settings.Language == model.LanguageEN {
			return fmt.Errorf("only successful Daily Digests with content can be delivered")
		}
		return fmt.Errorf("只有生成成功且包含内容的每日总览才能发送")
	}
	return s.deliverAndRecord(ctx, settings, item)
}

// continueExisting 根据持久化状态恢复生成、重试模型或重试投递。
func (s *Service) continueExisting(ctx context.Context, settings model.AppSettings, item model.DailyDigest) error {
	switch item.Status {
	case model.SummaryStatusPending:
		return s.startGeneration(ctx, item.ID, false, !item.DeliverySuppressed)
	case model.SummaryStatusRunning:
		return nil
	case model.SummaryStatusFailed:
		if item.NextRetryAt == nil || item.NextRetryAt.After(s.clock.Now()) || item.RetryCount >= settings.SummaryRetryLimit {
			return nil
		}
		return s.startGeneration(ctx, item.ID, true, !item.DeliverySuppressed)
	case model.SummaryStatusSucceeded:
		if !shouldAutomaticallyDeliver(item) {
			return nil
		}
		return s.deliverAndRecord(ctx, settings, item)
	default:
		return nil
	}
}

// startGeneration 先持久化运行状态，再用进程级上下文执行模型调用。
func (s *Service) startGeneration(ctx context.Context, id int64, retry bool, deliver bool) error {
	key := fmt.Sprintf("generation:%d", id)
	if !s.begin(key) {
		return nil
	}
	if retry {
		if err := s.store.DailyDigests.SetRetryRunning(ctx, id); err != nil {
			s.finish(key)
			return err
		}
	} else if err := s.store.DailyDigests.SetRunning(ctx, id, !deliver); err != nil {
		s.finish(key)
		return err
	}
	go func() {
		defer s.finish(key)
		runCtx := s.root
		if runCtx == nil {
			runCtx = context.Background()
		}
		if err := s.execute(runCtx, id, deliver); err != nil && !errors.Is(err, context.Canceled) {
			_ = s.failUnexpected(id, err)
		}
	}()
	return nil
}

// failUnexpected 将模型流程之外的任务级错误沉淀到历史记录。
func (s *Service) failUnexpected(id int64, err error) error {
	item, getErr := s.store.DailyDigests.GetByID(context.Background(), id)
	if getErr != nil {
		return getErr
	}
	now := s.clock.Now()
	item.Status = model.SummaryStatusFailed
	item.ErrorMessage = err.Error()
	item.NextRetryAt = nil
	item.CompletedAt = &now
	return s.store.DailyDigests.SaveResult(context.Background(), item)
}

// execute 保存模型结果，并在自动任务成功时执行一次 Telegram 投递。
func (s *Service) execute(ctx context.Context, id int64, deliver bool) error {
	item, err := s.store.DailyDigests.GetForGeneration(ctx, id)
	if err != nil {
		return err
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	result := generate(ctx, settings, item, s.openAITimeout, s.clock.Now())
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	if err := s.store.DailyDigests.SaveResult(ctx, result); err != nil {
		return err
	}
	if !deliver || !shouldAutomaticallyDeliver(result) {
		return nil
	}
	deliverySettings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return nil
	}
	deliverySettings.Language = settings.Language
	_ = s.deliverAndRecord(ctx, deliverySettings, result)
	return nil
}

// shouldAutomaticallyDeliver 排除已发送、无内容和要求人工确认的总览。
func shouldAutomaticallyDeliver(item model.DailyDigest) bool {
	return item.Status == model.SummaryStatusSucceeded &&
		item.DeliveredAt == nil &&
		item.DeliverySkippedReason == "" &&
		!item.DeliverySuppressed
}

// deliverAndRecord 将正文生成状态与 Telegram 投递状态分别持久化。
func (s *Service) deliverAndRecord(ctx context.Context, settings model.AppSettings, item model.DailyDigest) error {
	key := fmt.Sprintf("delivery:%d", item.ID)
	if !s.begin(key) {
		return nil
	}
	defer s.finish(key)
	if !botReady(settings) {
		return ErrBotUnavailable
	}
	message := buildDeliveryMessage(settings.Language, item)
	if err := s.botService.SendLongMessage(ctx, settings.BotToken, settings.BotTargetChatID, message); err != nil {
		_ = s.store.DailyDigests.MarkDeliveryFailed(context.Background(), item.ID, err.Error())
		return err
	}
	return s.store.DailyDigests.MarkDelivered(ctx, item.ID, s.clock.Now())
}

// collectSources 读取参与群组的最终摘要；仍有任务待运行或待重试时返回未就绪。
func (s *Service) collectSources(
	ctx context.Context,
	settings model.AppSettings,
	summaryDate string,
	participants []participant,
) ([]model.DailyDigestSource, bool, error) {
	sources := make([]model.DailyDigestSource, 0, len(participants))
	var waiting []string
	for _, selected := range participants {
		item, err := s.store.Summaries.GetByChatAndDate(ctx, selected.chatID, summaryDate)
		if errors.Is(err, pgx.ErrNoRows) {
			waiting = append(waiting, selected.chatTitle)
			continue
		}
		if err != nil {
			return nil, false, err
		}
		if !summaryTerminal(item, settings, summaryDate) {
			waiting = append(waiting, selected.chatTitle)
			continue
		}
		sources = append(sources, dailyDigestSource(selected, item))
	}
	if len(waiting) > 0 {
		if settings.Language == model.LanguageEN {
			return nil, false, fmt.Errorf("Summaries missing or still processing: %s: %w", strings.Join(waiting, ", "), ErrSourcesNotReady)
		}
		return nil, false, fmt.Errorf("以下群组的摘要尚未生成或仍在处理中：%s: %w", strings.Join(waiting, "、"), ErrSourcesNotReady)
	}
	return sources, true, nil
}

// dailyDigestSource 将成功摘要、空群和最终失败群组分为三类来源。
func dailyDigestSource(selected participant, item model.Summary) model.DailyDigestSource {
	source := model.DailyDigestSource{
		SummaryID: item.ID, ChatID: selected.chatID, ChatTitle: selected.chatTitle,
		SummaryStatus: item.Status, SourceMessageCount: item.SourceMessageCount,
		Content: item.Content, Model: item.Model,
	}
	switch {
	case item.Status == model.SummaryStatusFailed:
		source.OmissionReason = "generation_failed"
	case item.SourceMessageCount == 0:
		source.OmissionReason = "no_messages"
	case strings.TrimSpace(item.Content) == "":
		source.OmissionReason = "empty_content"
	default:
		source.Included = true
	}
	return source
}

// summaryTerminal 接受正式成功摘要和重试已结束的失败摘要，拒绝预览版。
func summaryTerminal(item model.Summary, settings model.AppSettings, summaryDate string) bool {
	switch item.Status {
	case model.SummaryStatusSucceeded:
		end, err := summaryDayEnd(summaryDate, settings.DefaultTimezone)
		return err == nil && !item.GeneratedAt.Before(end)
	case model.SummaryStatusFailed:
		return item.NextRetryAt == nil || item.RetryCount >= settings.SummaryRetryLimit
	default:
		return false
	}
}

// participantChats 将群级 Bot 选项解释为每日总览的参与范围。
func participantChats(chats []model.Chat) []model.Chat {
	result := make([]model.Chat, 0)
	for _, chat := range chats {
		if chat.SummaryEnabled && chat.DeliveryMode == model.DeliveryModeBot {
			result = append(result, chat)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Title == result[right].Title {
			return result[left].ID < result[right].ID
		}
		return result[left].Title < result[right].Title
	})
	return result
}

// participantsFromChats 提取重建来源所需的群组 ID 和标题快照。
func participantsFromChats(chats []model.Chat) []participant {
	result := make([]participant, 0, len(chats))
	for _, chat := range chats {
		result = append(result, participant{chatID: chat.ID, chatTitle: chat.Title})
	}
	return result
}

// allParticipantsDue 以参与群组中最晚的摘要时间作为总览起点。
func allParticipantsDue(now time.Time, chats []model.Chat, timezone string) bool {
	location, err := loadLocation(timezone)
	if err != nil {
		return false
	}
	localNow := now.In(location)
	for _, chat := range chats {
		scheduled, err := time.ParseInLocation("15:04", chat.SummaryTimeLocal, location)
		if err != nil {
			return false
		}
		dueAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), scheduled.Hour(), scheduled.Minute(), 0, 0, location)
		if localNow.Before(dueAt) {
			return false
		}
	}
	return true
}

// summaryDayEnd 按系统时区计算摘要日期的结束边界。
func summaryDayEnd(summaryDate string, timezone string) (time.Time, error) {
	location, err := loadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01-02", summaryDate, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse daily digest date %s: %w", summaryDate, err)
	}
	return start.AddDate(0, 0, 1), nil
}

// loadLocation 对空配置沿用进程时区，其余值必须是有效 IANA 时区。
func loadLocation(timezone string) (*time.Location, error) {
	if strings.TrimSpace(timezone) == "" {
		return time.Local, nil
	}
	return time.LoadLocation(timezone)
}

func botReady(settings model.AppSettings) bool {
	return settings.BotEnabled && strings.TrimSpace(settings.BotToken) != "" && strings.TrimSpace(settings.BotTargetChatID) != ""
}

// begin 阻止定时器和手动请求并发执行同一种任务操作。
func (s *Service) begin(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.inflight[key]; exists {
		return false
	}
	s.inflight[key] = struct{}{}
	return true
}

func (s *Service) finish(key string) {
	s.mu.Lock()
	delete(s.inflight, key)
	s.mu.Unlock()
}
