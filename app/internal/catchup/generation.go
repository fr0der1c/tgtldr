package catchup

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

const (
	catchUpStageOutputReserve = 1200
	catchUpMinimumInputBudget = 512
	catchUpMaximumMergeDepth  = 6
	catchUpSeparator          = "\n\n---\n\n"
)

type generationBudget struct {
	counter          llmcontext.Counter
	contextWindow    int
	stageInputBudget int
	stageRequestMax  int
	finalRequestMax  int
	finalReserve     int
	parallelism      int
}

type generationResult struct {
	content              string
	model                string
	chunkCount           int
	executionMode        string
	estimatedInputTokens int
	contextWindowTokens  int
	fallbackReason       string
}

// generate 优先将全部每日摘要一次交给模型，容量不足或超限时再分批归并。
func generate(
	ctx context.Context,
	settings model.AppSettings,
	sources []model.CatchUpSource,
	openAITimeout time.Duration,
) (generationResult, error) {
	modelName := strings.TrimSpace(settings.OpenAIModel)
	budget := resolveGenerationBudget(settings, modelName)
	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   modelName,
		Timeout: openAITimeout,
		Stream:  settings.OpenAIRequestMode != model.OpenAIRequestModeNonStream,
	})
	units := buildSourceUnits(sources, settings.Language)
	finalPrompt := buildFinalPrompt(settings.Language)
	fullInput := strings.Join(units, catchUpSeparator)
	directPlan := llmcontext.PlanRequest(
		budget.counter, budget.contextWindow, finalPrompt, fullInput, budget.finalReserve,
	)
	result := generationResult{
		model:                modelName,
		executionMode:        "chunked",
		estimatedInputTokens: directPlan.InputTokens,
		contextWindowTokens:  budget.contextWindow,
	}
	chunkBudget := budget.stageInputBudget

	if directPlan.Fits {
		response, err := callModel(ctx, client, settings, finalPrompt, fullInput, budget.finalRequestMax)
		if err == nil {
			result.content = strings.TrimSpace(response.Content)
			result.model = responseModel(response.Model, modelName)
			result.chunkCount = 1
			result.executionMode = "single"
			return result, nil
		}
		if !openai.IsContextLimitError(err) {
			return generationResult{}, err
		}
		result.executionMode = "fallback_chunked"
		result.fallbackReason = "context_limit_error"
		chunkBudget = minInt(chunkBudget, maxInt(catchUpMinimumInputBudget, directPlan.InputTokens/2))
	}

	stagePrompt := buildStagePrompt(settings.Language)
	partials, chunkCount, mergeBudget, err := generatePartials(ctx, client, settings, stagePrompt, units, chunkBudget, budget)
	if err != nil {
		return generationResult{}, err
	}
	response, err := mergePartials(ctx, client, settings, finalPrompt, partials, budget, mergeBudget, 0)
	if err != nil {
		return generationResult{}, err
	}
	result.content = strings.TrimSpace(response.Content)
	result.model = responseModel(response.Model, modelName)
	result.chunkCount = chunkCount
	return result, nil
}

// generatePartials 并行处理输入批次，并在上游报告超限时逐步缩小批次。
func generatePartials(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	stagePrompt string,
	units []string,
	initialBudget int,
	budget generationBudget,
) ([]string, int, int, error) {
	inputBudget := maxInt(catchUpMinimumInputBudget, initialBudget)
	for {
		batches := packSourceUnits(units, inputBudget, budget.counter)
		partials, err := runStageBatch(ctx, client, settings, stagePrompt, batches, budget)
		if err == nil {
			return partials, len(batches), inputBudget, nil
		}
		if !openai.IsContextLimitError(err) || inputBudget <= catchUpMinimumInputBudget {
			return nil, len(batches), inputBudget, err
		}
		inputBudget = maxInt(catchUpMinimumInputBudget, inputBudget/2)
	}
}

// packSourceUnits 切分超大每日摘要时重复来源头，确保每个片段仍可追溯。
func packSourceUnits(units []string, inputBudget int, counter llmcontext.Counter) []string {
	expanded := make([]string, 0, len(units))
	for _, unit := range units {
		if counter.Count(unit) <= inputBudget {
			expanded = append(expanded, unit)
			continue
		}
		marker := "\n每日摘要：\n"
		markerIndex := strings.Index(unit, marker)
		if markerIndex < 0 {
			marker = "\nDaily summary:\n"
			markerIndex = strings.Index(unit, marker)
		}
		if markerIndex < 0 {
			expanded = append(expanded, llmcontext.SplitTextToBudget(unit, inputBudget, counter)...)
			continue
		}
		bodyStart := markerIndex + len(marker)
		prefix := unit[:bodyStart]
		bodyBudget := inputBudget - counter.Count(prefix)
		if bodyBudget < 128 {
			expanded = append(expanded, llmcontext.SplitTextToBudget(unit, inputBudget, counter)...)
			continue
		}
		for _, piece := range llmcontext.SplitTextToBudget(unit[bodyStart:], bodyBudget, counter) {
			expanded = append(expanded, prefix+piece)
		}
	}
	return llmcontext.PackTextParts(expanded, inputBudget, counter, catchUpSeparator)
}

// runStageBatch 按配置并行度生成阶段笔记，输出索引与批次索引一致。
func runStageBatch(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	stagePrompt string,
	batches []string,
	budget generationBudget,
) ([]string, error) {
	partials := make([]string, len(batches))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(budget.parallelism)
	for index, batch := range batches {
		index := index
		batch := batch
		group.Go(func() error {
			response, err := callModel(groupCtx, client, settings, stagePrompt, batch, budget.stageRequestMax)
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

// mergePartials 仅在阶段结果仍超过上下文时增加归并层级。
func mergePartials(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	finalPrompt string,
	partials []string,
	budget generationBudget,
	inputBudgetCap int,
	depth int,
) (openai.ChatResponse, error) {
	if depth > catchUpMaximumMergeDepth {
		return openai.ChatResponse{}, fmt.Errorf("merge Catch Up: exceeded %d levels", catchUpMaximumMergeDepth)
	}
	input := strings.Join(partials, catchUpSeparator)
	plan := llmcontext.PlanRequest(budget.counter, budget.contextWindow, finalPrompt, input, budget.finalReserve)
	if inputBudgetCap > 0 {
		plan.InputBudget = minInt(plan.InputBudget, inputBudgetCap)
		plan.Fits = plan.InputTokens <= plan.InputBudget
	}
	if plan.Fits {
		response, err := callModel(ctx, client, settings, finalPrompt, input, budget.finalRequestMax)
		if err == nil || !openai.IsContextLimitError(err) {
			return response, err
		}
		plan.InputBudget = maxInt(catchUpMinimumInputBudget, plan.InputTokens/2)
	}

	mergeInputBudget := maxInt(catchUpMinimumInputBudget, plan.InputBudget)
	batches := llmcontext.PackTextParts(
		partials,
		mergeInputBudget,
		budget.counter,
		catchUpSeparator,
	)
	if len(batches) == 0 {
		return openai.ChatResponse{}, fmt.Errorf("merge Catch Up: no partials")
	}
	merged := make([]string, 0, len(batches))
	modelUsed := settings.OpenAIModel
	for _, batch := range batches {
		response, err := callModel(ctx, client, settings, finalPrompt, batch, budget.finalRequestMax)
		if err != nil {
			if openai.IsContextLimitError(err) && mergeInputBudget > catchUpMinimumInputBudget {
				return mergePartials(
					ctx, client, settings, finalPrompt, partials, budget,
					maxInt(catchUpMinimumInputBudget, mergeInputBudget/2), depth,
				)
			}
			return openai.ChatResponse{}, err
		}
		merged = append(merged, strings.TrimSpace(response.Content))
		modelUsed = responseModel(response.Model, modelUsed)
	}
	if len(merged) == 1 {
		return openai.ChatResponse{Content: merged[0], Model: modelUsed}, nil
	}
	return mergePartials(ctx, client, settings, finalPrompt, merged, budget, inputBudgetCap, depth+1)
}

// callModel 使用统一的温度与输出限制发送一次 Catch Up 模型请求。
func callModel(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	systemPrompt string,
	userPrompt string,
	maxOutput int,
) (openai.ChatResponse, error) {
	return client.Chat(ctx, openai.ChatRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  settings.OpenAITemperature,
		MaxOutput:    maxOutput,
	})
}

// resolveGenerationBudget 将全局模型设置转换成 Catch Up 的输入和输出预算。
func resolveGenerationBudget(settings model.AppSettings, modelName string) generationBudget {
	configuredContext := 0
	if settings.OpenAIContextWindowMode == model.ContextWindowModeManual {
		configuredContext = settings.OpenAIContextWindowTokens
	}
	contextWindow := llmcontext.ResolveContextWindow(modelName, configuredContext)
	counter := llmcontext.NewCounter(modelName)
	stageRequestMax := 0
	finalRequestMax := 0
	stageReserve := catchUpStageOutputReserve
	finalReserve := llmcontext.DefaultOutputReserve
	if settings.OpenAIOutputMode == model.OutputModeManual && settings.OpenAIMaxOutputToken > 0 {
		finalRequestMax = settings.OpenAIMaxOutputToken
		stageRequestMax = minInt(finalRequestMax, catchUpStageOutputReserve)
		stageReserve = stageRequestMax
		finalReserve = finalRequestMax
	}
	stagePlan := llmcontext.PlanRequest(counter, contextWindow, buildStagePrompt(settings.Language), "", stageReserve)
	parallelism := settings.SummaryParallelism
	if parallelism <= 0 {
		parallelism = 2
	}
	parallelism = maxInt(1, minInt(6, parallelism))
	return generationBudget{
		counter: counter, contextWindow: contextWindow,
		stageInputBudget: maxInt(catchUpMinimumInputBudget, stagePlan.InputBudget),
		stageRequestMax:  stageRequestMax, finalRequestMax: finalRequestMax,
		finalReserve: finalReserve, parallelism: parallelism,
	}
}

func responseModel(responseModelName, fallback string) string {
	if strings.TrimSpace(responseModelName) != "" {
		return responseModelName
	}
	return fallback
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
