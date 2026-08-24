package catchup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

const MaximumDateRangeDays = 90

var (
	ErrInvalidDateRange  = errors.New("invalid Catch Up date range")
	ErrDateRangeTooLong  = errors.New("Catch Up date range is too long")
	ErrDateRangeInFuture = errors.New("Catch Up date range includes today or a future date")
	ErrBotUnavailable    = errors.New("Telegram Bot is not configured")
)

type CreateInput struct {
	FromDate          string
	ToDate            string
	ChatIDs           []int64
	DeliveryRequested bool
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

// NewService 创建使用进程级上下文执行后台任务的 Catch Up 服务。
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

// RecoverInterrupted 在服务启动时结束上一次进程遗留的未完成任务。
func (s *Service) RecoverInterrupted(ctx context.Context) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	message := "快速回顾因应用重启而中断，请重新生成。"
	if settings.Language == model.LanguageEN {
		message = "Catch Up was interrupted by an application restart. Please create it again."
	}
	return s.store.CatchUps.FailInterrupted(ctx, message)
}

// Start 固化来源并启动后台生成任务，HTTP 请求结束不会取消任务。
func (s *Service) Start(ctx context.Context, input CreateInput) (model.CatchUp, error) {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return model.CatchUp{}, err
	}
	if err := ValidateDateRange(input.FromDate, input.ToDate, settings.DefaultTimezone, s.clock.Now()); err != nil {
		return model.CatchUp{}, err
	}
	if input.DeliveryRequested && !botReady(settings) {
		return model.CatchUp{}, ErrBotUnavailable
	}

	item, err := s.store.CatchUps.Create(
		ctx, input.FromDate, input.ToDate, input.ChatIDs, input.DeliveryRequested,
	)
	if err != nil {
		return model.CatchUp{}, err
	}
	if err := s.store.CatchUps.SetRunning(ctx, item.ID); err != nil {
		return model.CatchUp{}, err
	}
	item.Status = model.SummaryStatusRunning
	go s.run(item.ID)
	return item, nil
}

func (s *Service) Get(ctx context.Context, id int64) (model.CatchUp, error) {
	return s.store.CatchUps.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, page, pageSize int) (model.CatchUpListResponse, error) {
	return s.store.CatchUps.List(ctx, page, pageSize)
}

// RetryDelivery 只重新发送已经生成成功的 Catch Up，不会重新调用摘要模型。
func (s *Service) RetryDelivery(ctx context.Context, id int64) error {
	key := fmt.Sprintf("delivery:%d", id)
	if !s.begin(key) {
		return nil
	}
	defer s.finish(key)

	item, err := s.store.CatchUps.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item.Status != model.SummaryStatusSucceeded {
		return fmt.Errorf("only succeeded Catch Up documents can be delivered")
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if err := s.deliver(ctx, settings, item); err != nil {
		_ = s.store.CatchUps.MarkDeliveryFailed(ctx, id, err.Error())
		return err
	}
	return s.store.CatchUps.MarkDelivered(ctx, id, s.clock.Now())
}

// ValidateDateRange 按配置时区校验完整自然日范围，截止日不能包含今天。
func ValidateDateRange(fromDate, toDate, timezone string, now time.Time) error {
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return fmt.Errorf("load Catch Up timezone %s: %w", timezone, err)
	}
	from, err := time.ParseInLocation("2006-01-02", fromDate, location)
	if err != nil {
		return ErrInvalidDateRange
	}
	to, err := time.ParseInLocation("2006-01-02", toDate, location)
	if err != nil || to.Before(from) {
		return ErrInvalidDateRange
	}
	today := now.In(location)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	if !to.Before(today) {
		return ErrDateRangeInFuture
	}

	days := 0
	for current := from; !current.After(to); current = current.AddDate(0, 0, 1) {
		days++
		if days > MaximumDateRangeDays {
			return ErrDateRangeTooLong
		}
	}
	return nil
}

// run 串行执行指定任务，并把任务级异常沉淀为失败记录。
func (s *Service) run(id int64) {
	key := fmt.Sprintf("generation:%d", id)
	if !s.begin(key) {
		return
	}
	defer s.finish(key)
	ctx := s.root
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.execute(ctx, id); err != nil {
		now := s.clock.Now()
		_ = s.store.CatchUps.SaveResult(context.Background(), model.CatchUp{
			ID: id, Status: model.SummaryStatusFailed, ErrorMessage: err.Error(),
			CompletedAt: &now,
		})
	}
}

// execute 生成 Catch Up 并独立记录 Telegram 投递结果。
func (s *Service) execute(ctx context.Context, id int64) error {
	item, err := s.store.CatchUps.GetForGeneration(ctx, id)
	if err != nil {
		return err
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	generated, err := generate(ctx, settings, item.Sources, s.openAITimeout)
	now := s.clock.Now()
	item.CompletedAt = &now
	item.Model = settings.OpenAIModel
	if err != nil {
		item.Status = model.SummaryStatusFailed
		item.ErrorMessage = err.Error()
		return s.store.CatchUps.SaveResult(ctx, item)
	}
	item.Status = model.SummaryStatusSucceeded
	item.Content = generated.content
	item.Model = generated.model
	item.ChunkCount = generated.chunkCount
	item.ExecutionMode = generated.executionMode
	item.EstimatedInputTokens = generated.estimatedInputTokens
	item.ContextWindowTokens = generated.contextWindowTokens
	item.FallbackReason = generated.fallbackReason
	item.GeneratedAt = &now
	item.ErrorMessage = ""
	if err := s.store.CatchUps.SaveResult(ctx, item); err != nil {
		return err
	}
	if !item.DeliveryRequested {
		return nil
	}
	if err := s.deliver(ctx, settings, item); err != nil {
		_ = s.store.CatchUps.MarkDeliveryFailed(context.Background(), id, err.Error())
		return nil
	}
	_ = s.store.CatchUps.MarkDelivered(context.Background(), id, now)
	return nil
}

// deliver 使用任务完成时的全局 Bot 配置发送完整 Catch Up 正文。
func (s *Service) deliver(ctx context.Context, settings model.AppSettings, item model.CatchUp) error {
	if !botReady(settings) {
		return ErrBotUnavailable
	}
	message := buildDeliveryMessage(settings.Language, item)
	return s.botService.SendLongMessage(
		ctx, settings.BotToken, settings.BotTargetChatID, message,
	)
}

// buildDeliveryMessage 根据系统语言生成 Telegram 标题并附加完整正文。
func buildDeliveryMessage(language model.Language, item model.CatchUp) string {
	title := "快速回顾"
	if language == model.LanguageEN {
		title = "Catch Up"
	}
	header := fmt.Sprintf("**%s · %s – %s**", title, item.FromDate, item.ToDate)
	message := header
	if strings.TrimSpace(item.Content) != "" {
		message += "\n\n" + strings.TrimSpace(item.Content)
	}
	return message
}

func botReady(settings model.AppSettings) bool {
	return settings.BotEnabled &&
		strings.TrimSpace(settings.BotToken) != "" &&
		strings.TrimSpace(settings.BotTargetChatID) != ""
}

// begin 在进程内阻止同一 Catch Up 操作被并发重复执行。
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
