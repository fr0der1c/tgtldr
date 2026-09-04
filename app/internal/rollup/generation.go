package rollup

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
	stageOutputReserve = 1200
	minimumInputBudget = 512
	maximumMergeDepth  = 6
	separator          = "\n\n---\n\n"
)

type Source struct {
	Header  string
	Content string
}

type Prompts struct {
	Stage string
	Final string
}

type Result struct {
	Content              string
	Model                string
	ChunkCount           int
	ExecutionMode        string
	EstimatedInputTokens int
	ContextWindowTokens  int
	FallbackReason       string
}

type generationBudget struct {
	counter          llmcontext.Counter
	contextWindow    int
	stageInputBudget int
	stageRequestMax  int
	finalRequestMax  int
	finalReserve     int
	parallelism      int
}

// Generate 优先一次归并所有来源，容量不足或超限时再分批处理。
func Generate(
	ctx context.Context,
	settings model.AppSettings,
	prompts Prompts,
	sources []Source,
	openAITimeout time.Duration,
) (Result, error) {
	if len(sources) == 0 {
		return Result{}, fmt.Errorf("generate rollup: no sources")
	}
	modelName := strings.TrimSpace(settings.OpenAIModel)
	budget := resolveGenerationBudget(settings, modelName, prompts.Stage)
	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   modelName,
		Timeout: openAITimeout,
		Stream:  settings.OpenAIRequestMode != model.OpenAIRequestModeNonStream,
	})
	units := sourceUnits(sources)
	fullInput := strings.Join(units, separator)
	directPlan := llmcontext.PlanRequest(
		budget.counter, budget.contextWindow, prompts.Final, fullInput, budget.finalReserve,
	)
	result := Result{
		Model:                modelName,
		ExecutionMode:        "chunked",
		EstimatedInputTokens: directPlan.InputTokens,
		ContextWindowTokens:  budget.contextWindow,
	}
	chunkBudget := budget.stageInputBudget

	if directPlan.Fits {
		response, err := callModel(ctx, client, settings, prompts.Final, fullInput, budget.finalRequestMax)
		if err == nil {
			result.Content = strings.TrimSpace(response.Content)
			result.Model = responseModel(response.Model, modelName)
			result.ChunkCount = 1
			result.ExecutionMode = "single"
			return result, nil
		}
		if !openai.IsContextLimitError(err) {
			return Result{}, err
		}
		result.ExecutionMode = "fallback_chunked"
		result.FallbackReason = "context_limit_error"
		chunkBudget = min(chunkBudget, max(minimumInputBudget, directPlan.InputTokens/2))
	}

	partials, chunkCount, mergeBudget, err := generatePartials(
		ctx, client, settings, prompts.Stage, sources, chunkBudget, budget,
	)
	if err != nil {
		return Result{}, err
	}
	response, err := mergePartials(ctx, client, settings, prompts.Final, partials, budget, mergeBudget, 0)
	if err != nil {
		return Result{}, err
	}
	result.Content = strings.TrimSpace(response.Content)
	result.Model = responseModel(response.Model, modelName)
	result.ChunkCount = chunkCount
	return result, nil
}

// sourceUnits 将可重复的来源头与正文组合成模型输入单元。
func sourceUnits(sources []Source) []string {
	units := make([]string, 0, len(sources))
	for _, source := range sources {
		units = append(units, source.Header+strings.TrimSpace(source.Content))
	}
	return units
}

// generatePartials 并行处理输入批次，并在上游报告超限时逐步缩小批次。
func generatePartials(
	ctx context.Context,
	client *openai.Client,
	settings model.AppSettings,
	stagePrompt string,
	sources []Source,
	initialBudget int,
	budget generationBudget,
) ([]string, int, int, error) {
	inputBudget := max(minimumInputBudget, initialBudget)
	for {
		batches := packSources(sources, inputBudget, budget.counter)
		partials, err := runStageBatch(ctx, client, settings, stagePrompt, batches, budget)
		if err == nil {
			return partials, len(batches), inputBudget, nil
		}
		if !openai.IsContextLimitError(err) || inputBudget <= minimumInputBudget {
			return nil, len(batches), inputBudget, err
		}
		inputBudget = max(minimumInputBudget, inputBudget/2)
	}
}

// packSources 拆分超大正文时重复来源头，确保每个片段仍可追溯。
func packSources(sources []Source, inputBudget int, counter llmcontext.Counter) []string {
	expanded := make([]string, 0, len(sources))
	for _, source := range sources {
		unit := source.Header + strings.TrimSpace(source.Content)
		if counter.Count(unit) <= inputBudget {
			expanded = append(expanded, unit)
			continue
		}
		bodyBudget := inputBudget - counter.Count(source.Header)
		if bodyBudget < 128 {
			expanded = append(expanded, llmcontext.SplitTextToBudget(unit, inputBudget, counter)...)
			continue
		}
		for _, piece := range llmcontext.SplitTextToBudget(source.Content, bodyBudget, counter) {
			expanded = append(expanded, source.Header+piece)
		}
	}
	return llmcontext.PackTextParts(expanded, inputBudget, counter, separator)
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

// mergePartials 在阶段结果仍超过上下文时继续分层归并。
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
	if depth > maximumMergeDepth {
		return openai.ChatResponse{}, fmt.Errorf("merge rollup: exceeded %d levels", maximumMergeDepth)
	}
	input := strings.Join(partials, separator)
	plan := llmcontext.PlanRequest(budget.counter, budget.contextWindow, finalPrompt, input, budget.finalReserve)
	if inputBudgetCap > 0 {
		plan.InputBudget = min(plan.InputBudget, inputBudgetCap)
		plan.Fits = plan.InputTokens <= plan.InputBudget
	}
	if plan.Fits {
		response, err := callModel(ctx, client, settings, finalPrompt, input, budget.finalRequestMax)
		if err == nil || !openai.IsContextLimitError(err) {
			return response, err
		}
		plan.InputBudget = max(minimumInputBudget, plan.InputTokens/2)
	}

	mergeInputBudget := max(minimumInputBudget, plan.InputBudget)
	batches := llmcontext.PackTextParts(partials, mergeInputBudget, budget.counter, separator)
	if len(batches) == 0 {
		return openai.ChatResponse{}, fmt.Errorf("merge rollup: no partials")
	}
	merged := make([]string, 0, len(batches))
	modelUsed := settings.OpenAIModel
	for _, batch := range batches {
		response, err := callModel(ctx, client, settings, finalPrompt, batch, budget.finalRequestMax)
		if err != nil {
			if openai.IsContextLimitError(err) && mergeInputBudget > minimumInputBudget {
				return mergePartials(
					ctx, client, settings, finalPrompt, partials, budget,
					max(minimumInputBudget, mergeInputBudget/2), depth,
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

// callModel 统一沿用系统温度和输出长度设置。
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

// resolveGenerationBudget 将模型设置转换成多来源归并的输入和输出预算。
func resolveGenerationBudget(settings model.AppSettings, modelName string, stagePrompt string) generationBudget {
	configuredContext := 0
	if settings.OpenAIContextWindowMode == model.ContextWindowModeManual {
		configuredContext = settings.OpenAIContextWindowTokens
	}
	contextWindow := llmcontext.ResolveContextWindow(modelName, configuredContext)
	counter := llmcontext.NewCounter(modelName)
	stageRequestMax := 0
	finalRequestMax := 0
	stageReserve := stageOutputReserve
	finalReserve := llmcontext.DefaultOutputReserve
	if settings.OpenAIOutputMode == model.OutputModeManual && settings.OpenAIMaxOutputToken > 0 {
		finalRequestMax = settings.OpenAIMaxOutputToken
		stageRequestMax = min(finalRequestMax, stageOutputReserve)
		stageReserve = stageRequestMax
		finalReserve = finalRequestMax
	}
	stagePlan := llmcontext.PlanRequest(counter, contextWindow, stagePrompt, "", stageReserve)
	parallelism := settings.SummaryParallelism
	if parallelism <= 0 {
		parallelism = 2
	}
	parallelism = max(1, min(6, parallelism))
	return generationBudget{
		counter: counter, contextWindow: contextWindow,
		stageInputBudget: max(minimumInputBudget, stagePlan.InputBudget),
		stageRequestMax:  stageRequestMax, finalRequestMax: finalRequestMax,
		finalReserve: finalReserve, parallelism: parallelism,
	}
}

// responseModel 优先记录上游实际返回的模型名称。
func responseModel(responseModelName, fallback string) string {
	if strings.TrimSpace(responseModelName) != "" {
		return responseModelName
	}
	return fallback
}
