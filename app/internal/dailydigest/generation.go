package dailydigest

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
	"github.com/fr0der1c/tgtldr/app/internal/rollup"
)

// generate 根据有效来源数量选择空结果、直接复用或模型归并。
func generate(
	ctx context.Context,
	settings model.AppSettings,
	item model.DailyDigest,
	openAITimeout time.Duration,
	now time.Time,
) model.DailyDigest {
	item.CompletedAt = &now
	sources := buildRollupSources(item.Sources, settings.Language)
	if len(sources) == 0 {
		if item.OmittedChatCount > 0 {
			item.Status = model.SummaryStatusFailed
			item.ErrorMessage = "没有可用于每日总览的成功摘要。"
			if settings.Language == model.LanguageEN {
				item.ErrorMessage = "No successful chat summaries are available for this Daily Digest."
			}
			return item
		}
		item.Status = model.SummaryStatusSucceeded
		item.Content = emptyDigestContent(settings.Language)
		item.ExecutionMode = "no_content"
		item.DeliverySkippedReason = "no_content"
		item.GeneratedAt = &now
		return item
	}
	if len(sources) == 1 {
		item.Status = model.SummaryStatusSucceeded
		item.Content = appendOmissionNotice(sources[0].Content, item.Sources, settings.Language)
		item.Model = sourceModel(item.Sources, settings.OpenAIModel)
		item.ChunkCount = 1
		item.ExecutionMode = "passthrough"
		item.GeneratedAt = &now
		return item
	}

	result, err := rollup.Generate(ctx, settings, rollup.Prompts{
		Stage: buildStagePrompt(settings.Language),
		Final: buildFinalPrompt(settings.Language),
	}, sources, openAITimeout)
	if err != nil {
		item.Status = model.SummaryStatusFailed
		item.ErrorMessage = err.Error()
		if openai.IsRetryableError(err) {
			item.NextRetryAt = nextRetryAt(settings, item.RetryCount, now)
		}
		return item
	}
	item.Status = model.SummaryStatusSucceeded
	item.Content = appendOmissionNotice(result.Content, item.Sources, settings.Language)
	item.Model = result.Model
	item.ChunkCount = result.ChunkCount
	item.ExecutionMode = result.ExecutionMode
	item.EstimatedInputTokens = result.EstimatedInputTokens
	item.ContextWindowTokens = result.ContextWindowTokens
	item.FallbackReason = result.FallbackReason
	item.GeneratedAt = &now
	return item
}

// sourceModel 在直通模式下记录来源实际使用的模型。
func sourceModel(sources []model.DailyDigestSource, fallback string) string {
	for _, source := range sources {
		if source.Included && strings.TrimSpace(source.Model) != "" {
			return source.Model
		}
	}
	return fallback
}

// nextRetryAt 在剩余重试次数内计算下一次模型调用时间。
func nextRetryAt(settings model.AppSettings, retryCount int, now time.Time) *time.Time {
	if settings.SummaryRetryLimit <= 0 || retryCount >= settings.SummaryRetryLimit {
		return nil
	}
	delay := model.SummaryRetryDelay(retryCount + 1)
	next := now.Add(delay)
	return &next
}

var deliverySourceMarker = regexp.MustCompile(`[ \t]*\[S[0-9]{3,}\]`)

// buildDeliveryMessage 生成带日期标题的 Telegram Markdown 消息。
func buildDeliveryMessage(language model.Language, item model.DailyDigest) string {
	title := "每日总览"
	if language == model.LanguageEN {
		title = "Daily Digest"
	}
	header := fmt.Sprintf("**%s · %s**", title, item.SummaryDate)
	if strings.TrimSpace(item.Content) == "" {
		return header
	}
	return header + "\n\n" + strings.TrimSpace(deliverySourceMarker.ReplaceAllString(item.Content, ""))
}
