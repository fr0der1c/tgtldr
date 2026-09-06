package summary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
	"golang.org/x/sync/errgroup"
)

const maximumMergeDepth = 6

// generateDailySummary 优先使用完整上下文生成日报，容量不足时再执行分块与合并。
func (s *Service) generateDailySummary(
	ctx context.Context,
	settings model.AppSettings,
	chat model.Chat,
	location *time.Location,
	messages []model.Message,
	messageLookup map[int]model.Message,
	result model.Summary,
) (model.Summary, error) {
	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   result.Model,
		Timeout: s.openAITimeout,
		Stream:  settings.OpenAIRequestMode != model.OpenAIRequestModeNonStream,
	})
	stagePrompt := buildStagePrompt(settings.Language, chat.SummaryContext, chat.SummaryPrompt)
	finalPrompt := buildFinalPrompt(settings.Language, chat.SummaryContext, chat.SummaryPrompt)
	budget := resolveSummaryBudget(settings, result.Model, stagePrompt)
	fullTranscript := BuildTranscript(messages, messageLookup, location, settings.Language)
	directPlan := llmcontext.PlanRequest(
		budget.Counter,
		budget.ContextWindow,
		finalPrompt,
		fullTranscript,
		budget.FinalReserve,
	)

	chunkBudget := budget.ChunkTokenBudget
	if directPlan.Fits {
		response, err := callDailyModel(ctx, client, settings, result.Model, "direct", nil, finalPrompt, fullTranscript, budget.FinalRequestMax)
		if err == nil {
			result.ChunkCount = 1
			return completeDailySummary(result, response), nil
		}
		if !openai.IsContextLimitError(err) {
			return failDailySummary(result, err), nil
		}
		chunkBudget = min(chunkBudget, max(minimumChunkTokenBudget, directPlan.InputTokens/2))
	}

	partials, chunks, mergeBudget, err := generateDailyPartials(
		ctx,
		client,
		settings,
		result.Model,
		stagePrompt,
		messages,
		messageLookup,
		location,
		chunkBudget,
		budget,
	)
	result.ChunkCount = len(chunks)
	if err != nil {
		return failDailySummary(result, err), nil
	}

	response, err := mergeDailyPartials(ctx, client, settings, result.Model, finalPrompt, partials, budget, mergeBudget, 0)
	if err != nil {
		return failDailySummary(result, err), nil
	}
	return completeDailySummary(result, response), nil
}

// generateDailyPartials 并行生成阶段摘要；若上游仍报告超限，则缩小分块后重试。
func generateDailyPartials(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	modelName string,
	stagePrompt string,
	messages []model.Message,
	messageLookup map[int]model.Message,
	location *time.Location,
	initialBudget int,
	budget summaryBudget,
) ([]string, []Chunk, int, error) {
	chunkBudget := max(minimumChunkTokenBudget, initialBudget)
	for {
		chunks := SplitMessagesWithCounter(messages, chunkBudget, budget.Counter)
		partials, err := runDailyChunkBatch(ctx, client, settings, modelName, stagePrompt, chunks, messageLookup, location, budget)
		if err == nil {
			return partials, chunks, chunkBudget, nil
		}
		if !openai.IsContextLimitError(err) || chunkBudget <= minimumChunkTokenBudget {
			return nil, chunks, chunkBudget, err
		}
		chunkBudget = max(minimumChunkTokenBudget, chunkBudget/2)
	}
}

// runDailyChunkBatch 按配置并行度生成一轮阶段摘要，并保持结果与分块顺序一致。
func runDailyChunkBatch(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	modelName string,
	stagePrompt string,
	chunks []Chunk,
	messageLookup map[int]model.Message,
	location *time.Location,
	budget summaryBudget,
) ([]string, error) {
	partials := make([]string, len(chunks))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(budget.Parallelism)
	for index, chunk := range chunks {
		index := index
		chunk := chunk
		group.Go(func() error {
			transcript := BuildTranscript(chunk.Messages, messageLookup, location, settings.Language)
			response, err := callDailyModel(groupCtx, client, settings, modelName, "chunk", &index, stagePrompt, transcript, budget.StageRequestMax)
			if err != nil {
				return err
			}
			partials[index] = strings.TrimSpace(response.Content)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return partials, nil
}

// mergeDailyPartials 在最终输入仍过大时按需增加合并层级，直到可以生成最终日报。
func mergeDailyPartials(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	modelName string,
	finalPrompt string,
	partials []string,
	budget summaryBudget,
	inputBudgetCap int,
	depth int,
) (openai.ChatResponse, error) {
	if depth > maximumMergeDepth {
		return openai.ChatResponse{}, fmt.Errorf("merge daily summary: exceeded %d levels", maximumMergeDepth)
	}
	finalInput := strings.Join(partials, "\n\n---\n\n")
	plan := llmcontext.PlanRequest(budget.Counter, budget.ContextWindow, finalPrompt, finalInput, budget.FinalReserve)
	if inputBudgetCap > 0 {
		plan.InputBudget = min(plan.InputBudget, inputBudgetCap)
		plan.Fits = plan.InputTokens <= plan.InputBudget
	}
	if plan.Fits {
		response, err := callDailyModel(ctx, client, settings, modelName, mergeStageName(depth), nil, finalPrompt, finalInput, budget.FinalRequestMax)
		if err == nil || !openai.IsContextLimitError(err) {
			return response, err
		}
		plan.InputBudget = max(minimumChunkTokenBudget, plan.InputTokens/2)
	}

	mergeInputBudget := max(minimumChunkTokenBudget, plan.InputBudget)
	batches := llmcontext.PackTextParts(partials, mergeInputBudget, budget.Counter, "\n\n---\n\n")
	if len(batches) == 0 {
		return openai.ChatResponse{}, fmt.Errorf("merge daily summary: no partials")
	}
	merged := make([]string, 0, len(batches))
	modelUsed := modelName
	for index, batch := range batches {
		response, err := callDailyModel(ctx, client, settings, modelName, mergeStageName(depth+1), &index, finalPrompt, batch, budget.FinalRequestMax)
		if err != nil {
			if openai.IsContextLimitError(err) && mergeInputBudget > minimumChunkTokenBudget {
				return mergeDailyPartials(
					ctx, client, settings, modelName, finalPrompt, partials, budget,
					max(minimumChunkTokenBudget, mergeInputBudget/2), depth,
				)
			}
			return openai.ChatResponse{}, err
		}
		merged = append(merged, strings.TrimSpace(response.Content))
		if strings.TrimSpace(response.Model) != "" {
			modelUsed = response.Model
		}
	}
	if len(merged) == 1 {
		return openai.ChatResponse{Content: merged[0], Model: modelUsed}, nil
	}
	return mergeDailyPartials(ctx, client, settings, modelName, finalPrompt, merged, budget, inputBudgetCap, depth+1)
}

// callDailyModel 发送一次摘要请求，并保存发生错误时所需的复现上下文。
func callDailyModel(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	modelName string,
	stage string,
	chunkIndex *int,
	systemPrompt string,
	userPrompt string,
	maxOutput int,
) (openai.ChatResponse, error) {
	snapshot := buildOpenAIRequestSnapshot(openAIRequestContextInput{
		Stage: stage, ChunkIndex: chunkIndex, BaseURL: settings.OpenAIBaseURL,
		Model: modelName, Temperature: settings.OpenAITemperature, MaxOutput: maxOutput,
		SystemPrompt: systemPrompt, UserPrompt: userPrompt,
	})
	response, err := client.Chat(ctx, openai.ChatRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  settings.OpenAITemperature,
		MaxOutput:    maxOutput,
	})
	if err != nil {
		return openai.ChatResponse{}, wrapOpenAIRequestError(err, snapshot)
	}
	return response, nil
}

// completeDailySummary 写入最终正文，并保留上游实际返回的模型名称。
func completeDailySummary(result model.Summary, response openai.ChatResponse) model.Summary {
	result.Content = strings.TrimSpace(response.Content)
	if strings.TrimSpace(response.Model) != "" {
		result.Model = response.Model
		result.ReturnedModel = strings.TrimSpace(response.Model)
	}
	return result
}

// failDailySummary 将模型错误及其请求快照写入可持久化摘要结果。
func failDailySummary(result model.Summary, err error) model.Summary {
	result.Status = model.SummaryStatusFailed
	result.ErrorMessage = err.Error()
	result.ErrorContext = openAIErrorContext(err)
	result.ErrorSystemPrompt = openAIErrorSystemPrompt(err)
	result.ErrorUserPrompt = openAIErrorUserPrompt(err)
	result.RetryableError = openAIErrorRetryable(err)
	return result
}

func mergeStageName(depth int) string {
	if depth == 0 {
		return "final"
	}
	return fmt.Sprintf("merge-%d", depth)
}
