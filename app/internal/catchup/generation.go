package catchup

import (
	"context"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/rollup"
)

type generationResult struct {
	content              string
	model                string
	chunkCount           int
	executionMode        string
	estimatedInputTokens int
	contextWindowTokens  int
	fallbackReason       string
}

// generate 使用通用多来源归并器生成快速回顾。
func generate(
	ctx context.Context,
	settings model.AppSettings,
	sources []model.CatchUpSource,
	openAITimeout time.Duration,
) (generationResult, error) {
	result, err := rollup.Generate(ctx, settings, rollup.Prompts{
		Stage: buildStagePrompt(settings.Language),
		Final: buildFinalPrompt(settings.Language),
	}, buildRollupSources(sources, settings.Language), openAITimeout)
	if err != nil {
		return generationResult{}, err
	}
	return generationResult{
		content: result.Content, model: result.Model, chunkCount: result.ChunkCount,
		executionMode: result.ExecutionMode, estimatedInputTokens: result.EstimatedInputTokens,
		contextWindowTokens: result.ContextWindowTokens, fallbackReason: result.FallbackReason,
	}, nil
}
