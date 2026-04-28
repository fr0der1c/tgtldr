package summary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/frederic/tgtldr/app/internal/clock"
	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/frederic/tgtldr/app/internal/openai"
	"github.com/frederic/tgtldr/app/internal/store"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	store         *store.Store
	clock         clock.Clock
	openAITimeout time.Duration
}

func NewService(st *store.Store, c clock.Clock, openAITimeout time.Duration) *Service {
	return &Service{store: st, clock: c, openAITimeout: openAITimeout}
}

func (s *Service) BuildContextPreview(ctx context.Context, summary model.Summary) (model.SummaryContextPreview, error) {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return model.SummaryContextPreview{}, err
	}

	chat, err := s.store.Chats.GetByID(ctx, summary.ChatID)
	if err != nil {
		return model.SummaryContextPreview{}, err
	}

	timezone := resolveSummaryTimezone(chat, settings.DefaultTimezone)
	location, err := loadLocation(timezone)
	if err != nil {
		return model.SummaryContextPreview{}, err
	}
	start, end, err := dayRange(summary.SummaryDate, timezone)
	if err != nil {
		return model.SummaryContextPreview{}, err
	}

	messages, err := s.store.Messages.ListForRange(ctx, chat.ID, start, end)
	if err != nil {
		return model.SummaryContextPreview{}, err
	}

	filteredMessages, messageLookup, err := s.prepareMessages(ctx, chat, messages)
	if err != nil {
		return model.SummaryContextPreview{}, err
	}
	stagePrompt := buildStagePrompt(settings.Language, chat.SummaryContext, chat.SummaryPrompt)
	finalPrompt := buildFinalPrompt(settings.Language, chat.SummaryContext, chat.SummaryPrompt)
	budget := resolveSummaryBudget(settings, resolveSummaryModel(chat, settings), stagePrompt)
	chunks := SplitMessages(filteredMessages, budget.ChunkTokenBudget)
	preview := model.SummaryContextPreview{
		SummaryID:        summary.ID,
		ChatID:           summary.ChatID,
		SummaryDate:      summary.SummaryDate,
		Model:            resolveSummaryModel(chat, settings),
		SystemPrompt:     stagePrompt,
		FinalPrompt:      finalPrompt,
		MessageCount:     len(filteredMessages),
		ChunkCount:       len(chunks),
		FinalInputNotice: finalInputNotice(settings.Language),
		PreviewNotice:    previewNotice(settings.Language),
	}

	for _, chunk := range chunks {
		preview.Chunks = append(preview.Chunks, model.SummaryContextChunk{
			Index:        chunk.Index,
			MessageCount: len(chunk.Messages),
			Content:      BuildTranscript(chunk.Messages, messageLookup, location, settings.Language),
		})
	}
	if len(chunks) <= 1 {
		preview.FinalPrompt = ""
		preview.FinalInputNotice = ""
	}
	return preview, nil
}

func (s *Service) RunDailySummary(ctx context.Context, chat model.Chat, date string) (model.Summary, error) {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return model.Summary{}, err
	}

	timezone := resolveSummaryTimezone(chat, settings.DefaultTimezone)
	location, err := loadLocation(timezone)
	if err != nil {
		return model.Summary{}, err
	}
	start, end, err := dayRange(date, timezone)
	if err != nil {
		return model.Summary{}, err
	}

	messages, err := s.store.Messages.ListForRange(ctx, chat.ID, start, end)
	if err != nil {
		return model.Summary{}, err
	}
	filteredMessages, messageLookup, err := s.prepareMessages(ctx, chat, messages)
	if err != nil {
		return model.Summary{}, err
	}

	summary := model.Summary{
		ChatID:             chat.ID,
		SummaryDate:        date,
		Status:             model.SummaryStatusSucceeded,
		Model:              resolveSummaryModel(chat, settings),
		SourceMessageCount: len(filteredMessages),
		GeneratedAt:        s.clock.Now(),
	}
	if len(filteredMessages) == 0 {
		summary.Content = emptySummaryContent(settings.Language)
		return summary, nil
	}

	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   resolveSummaryModel(chat, settings),
		Timeout: s.openAITimeout,
	})

	stagePrompt := buildStagePrompt(settings.Language, chat.SummaryContext, chat.SummaryPrompt)
	finalPrompt := buildFinalPrompt(settings.Language, chat.SummaryContext, chat.SummaryPrompt)
	budget := resolveSummaryBudget(settings, resolveSummaryModel(chat, settings), stagePrompt)
	chunks := SplitMessages(filteredMessages, budget.ChunkTokenBudget)
	summary.ChunkCount = len(chunks)

	partials := make([]string, len(chunks))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(budget.Parallelism)

	for index, chunk := range chunks {
		index := index
		chunk := chunk
		group.Go(func() error {
			transcript := BuildTranscript(chunk.Messages, messageLookup, location, settings.Language)
			resp, err := client.Chat(groupCtx, openai.ChatRequest{
				SystemPrompt: stagePrompt,
				UserPrompt:   transcript,
				Temperature:  settings.OpenAITemperature,
				MaxOutput:    budget.StageRequestMax,
			})
			if err != nil {
				return err
			}
			partials[index] = strings.TrimSpace(resp.Content)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		summary.Status = model.SummaryStatusFailed
		summary.ErrorMessage = err.Error()
		return summary, nil
	}

	finalInput := strings.Join(partials, "\n\n---\n\n")
	finalResp, err := client.Chat(ctx, openai.ChatRequest{
		SystemPrompt: finalPrompt,
		UserPrompt:   finalInput,
		Temperature:  settings.OpenAITemperature,
		MaxOutput:    budget.FinalRequestMax,
	})
	if err != nil {
		summary.Status = model.SummaryStatusFailed
		summary.ErrorMessage = err.Error()
		return summary, nil
	}

	summary.Content = strings.TrimSpace(finalResp.Content)
	summary.Model = finalResp.Model
	return summary, nil
}

func resolveSummaryModel(chat model.Chat, settings model.AppSettings) string {
	if strings.TrimSpace(chat.ModelOverride) != "" {
		return strings.TrimSpace(chat.ModelOverride)
	}
	return settings.OpenAIModel
}

func resolveSummaryTimezone(chat model.Chat, fallback string) string {
	if timezone := strings.TrimSpace(chat.SummaryTimezone); timezone != "" {
		return timezone
	}
	if timezone := strings.TrimSpace(fallback); timezone != "" {
		return timezone
	}
	return time.Local.String()
}

func loadLocation(timezone string) (*time.Location, error) {
	if strings.TrimSpace(timezone) == "" {
		return time.Local, nil
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load location %s: %w", timezone, err)
	}
	return location, nil
}

func dayRange(date string, timezone string) (time.Time, time.Time, error) {
	location, err := loadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse date %s: %w", date, err)
	}
	end := start.Add(24 * time.Hour)
	return start.UTC(), end.UTC(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Service) prepareMessages(ctx context.Context, chat model.Chat, messages []model.Message) ([]model.Message, map[int]model.Message, error) {
	lookup := make(map[int]model.Message, len(messages))
	for _, message := range messages {
		lookup[message.TelegramMessageID] = message
	}

	missingReplyIDs := make([]int, 0)
	for _, message := range messages {
		if message.ReplyToMessageID == 0 {
			continue
		}
		if _, ok := lookup[message.ReplyToMessageID]; ok {
			continue
		}
		missingReplyIDs = append(missingReplyIDs, message.ReplyToMessageID)
	}

	if len(missingReplyIDs) > 0 && s.store != nil && s.store.Messages != nil {
		referenced, err := s.store.Messages.LookupByTelegramIDs(ctx, chat.ID, uniqueInts(missingReplyIDs))
		if err != nil {
			return nil, nil, err
		}
		for messageID, message := range referenced {
			lookup[messageID] = message
		}
	}

	filtered := make([]model.Message, 0, len(messages))
	for _, message := range messages {
		if shouldSkipMessage(message, chat) {
			continue
		}
		if strings.TrimSpace(message.SummaryText()) == "" {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered, lookup, nil
}

func shouldSkipMessage(message model.Message, chat model.Chat) bool {
	if !chat.KeepBotMessages && message.SenderIsBot {
		return true
	}
	if matchesFilteredSender(message, chat.FilteredSenders) {
		return true
	}
	return matchesFilteredKeyword(message, chat.FilteredKeywords)
}

func matchesFilteredSender(message model.Message, filters []string) bool {
	if len(filters) == 0 {
		return false
	}

	name := normalizeFilterToken(message.SenderName)
	username := normalizeFilterToken(message.SenderUsername)

	for _, filter := range filters {
		target := normalizeFilterToken(filter)
		if target == "" {
			continue
		}
		if target == name || target == username {
			return true
		}
		if strings.HasPrefix(target, "@") && strings.TrimPrefix(target, "@") == username {
			return true
		}
	}
	return false
}

func matchesFilteredKeyword(message model.Message, filters []string) bool {
	if len(filters) == 0 {
		return false
	}

	text := normalizeFilterToken(message.SummaryText())
	if text == "" {
		return false
	}

	for _, filter := range filters {
		target := normalizeFilterToken(filter)
		if target == "" {
			continue
		}
		if strings.Contains(text, target) {
			return true
		}
	}
	return false
}

func normalizeFilterToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func uniqueInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
