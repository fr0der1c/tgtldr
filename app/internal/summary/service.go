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
	stagePrompt := buildStagePrompt(chat.SummaryContext, chat.SummaryPrompt)
	finalPrompt := buildFinalPrompt(chat.SummaryContext, chat.SummaryPrompt)
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
		FinalInputNotice: "最终合并输入来自各分块的阶段摘要。由于系统当前不会持久化阶段摘要快照，这里无法精确回放合并输入。",
		PreviewNotice:    "该预览会基于当前规则重建每个分块发送给 AI 的原始消息上下文。",
	}

	for _, chunk := range chunks {
		preview.Chunks = append(preview.Chunks, model.SummaryContextChunk{
			Index:        chunk.Index,
			MessageCount: len(chunk.Messages),
			Content:      BuildTranscript(chunk.Messages, messageLookup, location),
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
		summary.Content = "该日期没有可用于生成摘要的消息。"
		return summary, nil
	}

	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   resolveSummaryModel(chat, settings),
		Timeout: s.openAITimeout,
	})

	stagePrompt := buildStagePrompt(chat.SummaryContext, chat.SummaryPrompt)
	finalPrompt := buildFinalPrompt(chat.SummaryContext, chat.SummaryPrompt)
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
			transcript := BuildTranscript(chunk.Messages, messageLookup, location)
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

func buildStagePrompt(summaryContext string, prompt string) string {
	base := `
你是 TGTLDR 的阶段摘要器。你将阅读一段 Telegram 群聊记录，并提炼其中真正有信息价值的讨论内容。

这个群聊可能是自由发散讨论，而不是正式协作场景。你的目标不是机械复述聊天内容，而是提炼：
1. 这一段里主要在讨论哪些话题
2. 每个话题中大家表达了哪些观点和判断
3. 是否形成了相对明确的共识
4. 是否存在明显分歧或尚无定论的内容
5. 哪些信息只是零散提及，但可能值得注意

请优先关注：
- 被多人讨论的话题
- 对某个对象、现象、产品、服务、事件或观点的评价
- 群体判断、经验结论、使用反馈、倾向性意见
- 明显的正面反馈、负面反馈和变化趋势
- 带有上下文承接关系的回复消息

请忽略或弱化：
- 寒暄、玩笑、表情、灌水
- 没有信息增量的短回复
- 无法独立理解、且没有补充信息的碎片化内容
- 纯重复表达

如果消息带有 reply_to 和 reply_excerpt，请结合它理解上下文，不要孤立理解回复内容。

请使用中文输出，并按以下结构整理：

## 主要话题
- 列出这一段中出现的主要话题

## 分话题讨论摘要
### 话题：<名称>
- 讨论焦点：
- 主要观点：
- 初步判断：
- 分歧或未定点：

## 零散但值得注意的信息
- 列出提及较少但可能有参考价值的信息
`
	return buildSystemPrompt(base, summaryContext, prompt)
}

func buildFinalPrompt(summaryContext string, prompt string) string {
	base := `
你是 TGTLDR 的最终摘要器。你会收到多个阶段摘要，请将它们整理成一份适合用户快速阅读的中文群聊日报。

这个群聊可能是自由讨论群，而不是任务协作群。请不要强行提炼待办事项、行动项或正式结论，除非讨论中确实已经形成明确结果。

你的目标是帮助用户快速了解：
1. 今天主要讨论了哪些话题
2. 每个话题下，大家的主要观点和群体判断是什么
3. 哪些内容已经形成较明确的共识
4. 哪些内容存在分歧或信息不足
5. 哪些零散信息值得顺带关注

写作要求：
1. 优先提炼“话题”和“判断”，不要机械复述聊天过程
2. 合并重复信息，避免重复表达
3. 如果某个判断样本不足或存在明显争议，要明确说明
4. 不要把零散消息包装成确定事实
5. 语言简洁、直接，适合日报阅读

请按以下格式输出：

## 今日主要结论
- 用 3-6 条总结今天最值得关注的信息和判断

## 分话题总结

### <话题名称>
- 讨论内容：
- 群内主要观点：
- 当前判断：
- 分歧或不确定点：

### <话题名称>
- 讨论内容：
- 群内主要观点：
- 当前判断：
- 分歧或不确定点：

## 零散但值得注意的信息
- 列出提及较少但可能有参考价值的信息

## 仍不确定的信息
- 列出样本不足、无法形成稳定判断的内容
`
	return buildSystemPrompt(base, summaryContext, prompt)
}

func buildSystemPrompt(base string, summaryContext string, prompt string) string {
	sections := []string{strings.TrimSpace(base)}

	if contextText := strings.TrimSpace(summaryContext); contextText != "" {
		sections = append(sections, "群聊背景：\n"+contextText)
	}

	if extraPrompt := strings.TrimSpace(prompt); extraPrompt != "" {
		sections = append(sections, "额外要求：\n"+extraPrompt)
	}

	return strings.Join(sections, "\n\n")
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
