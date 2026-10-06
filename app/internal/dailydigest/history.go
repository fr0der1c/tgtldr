package dailydigest

import (
	"fmt"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/rollup"
)

const (
	historyDays            = 7
	maximumHistoryTokens   = 6000
	minimumHistoryExcerpt  = 128
	digestContextSeparator = "\n\n---\n\n"
)

// buildFinalContext 优先保留当天完整来源，在有限预算内加入由近到远的历史总览。
func buildFinalContext(settings model.AppSettings, summaryDate string, history []model.DailyDigest, sources []rollup.Source) string {
	dateHeader := fmt.Sprintf("总览日期：%s", summaryDate)
	historyHeader := "历史总览（仅供对照，前 7 个自然日内的可用记录，可能缺失或截断；不是当天来源）："
	truncated := "[历史内容因预算截断]"
	if settings.Language == model.LanguageEN {
		dateHeader = fmt.Sprintf("Digest date: %s", summaryDate)
		historyHeader = "Historical digests (comparison only; available records from the preceding 7 calendar days may be missing or truncated; not current-day sources):"
		truncated = "[Historical content truncated to fit the budget]"
	}
	if len(history) == 0 {
		return dateHeader
	}

	header := dateHeader + "\n\n" + historyHeader + "\n\n"
	counter := llmcontext.NewCounter(settings.OpenAIModel)
	configuredWindow := 0
	if settings.OpenAIContextWindowMode == model.ContextWindowModeManual {
		configuredWindow = settings.OpenAIContextWindowTokens
	}
	window := llmcontext.ResolveContextWindow(settings.OpenAIModel, configuredWindow)
	reserve := llmcontext.DefaultOutputReserve
	if settings.OpenAIOutputMode == model.OutputModeManual && settings.OpenAIMaxOutputToken > 0 {
		reserve = settings.OpenAIMaxOutputToken
	}
	units := make([]string, 0, len(sources))
	for _, source := range sources {
		units = append(units, source.Header+strings.TrimSpace(source.Content))
	}
	input := strings.Join(units, digestContextSeparator) + digestContextSeparator + dateHeader
	plan := llmcontext.PlanRequest(counter, window, buildFinalPrompt(settings.Language), input, reserve)
	historyBudget := min(maximumHistoryTokens, max(0, plan.InputBudget-counter.Count(header))/4)
	// 当天可直接处理时，缩减历史而不为历史额外触发分块。
	if plan.Fits {
		historyBudget = min(historyBudget, max(0, plan.InputBudget-plan.InputTokens-counter.Count("\n\n"+historyHeader+"\n\n")))
	}
	blocks := boundedHistory(history, counter, historyBudget, truncated)
	if len(blocks) == 0 {
		return dateHeader
	}
	return header + strings.Join(blocks, digestContextSeparator)
}

// boundedHistory 按输入顺序填充预算；调用方提供由近到远的总览，截断时标记不完整并移除旧来源编号。
func boundedHistory(history []model.DailyDigest, counter llmcontext.Counter, tokenBudget int, truncated string) []string {
	blocks := make([]string, 0, len(history))
	for _, item := range history {
		content := strings.TrimSpace(deliverySourceMarker.ReplaceAllString(item.Content, ""))
		if content == "" {
			continue
		}
		block := item.SummaryDate + "\n" + content
		candidate := strings.Join(append(blocks, block), digestContextSeparator)
		if counter.Count(candidate) <= tokenBudget {
			blocks = append(blocks, block)
			continue
		}
		remaining := tokenBudget - counter.Count(strings.Join(blocks, digestContextSeparator)) - counter.Count(digestContextSeparator+"\n"+truncated)
		if remaining >= minimumHistoryExcerpt {
			parts := llmcontext.SplitTextToBudget(block, remaining, counter)
			excerpt := parts[0] + "\n" + truncated
			candidate = strings.Join(append(blocks, excerpt), digestContextSeparator)
			if counter.Count(candidate) <= tokenBudget {
				blocks = append(blocks, excerpt)
			}
		}
		break
	}
	return blocks
}
