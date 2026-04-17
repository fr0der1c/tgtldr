package summary

import (
	"strings"

	"github.com/frederic/tgtldr/app/internal/model"
)

const (
	defaultStageOutputReserve = 1200
	defaultChunkSafetyMargin  = 1800
	defaultChunkContextRatio  = 0.5
	minChunkTokenBudget       = 2400
)

type summaryBudget struct {
	ChunkTokenBudget int
	Parallelism      int
	StageRequestMax  int
	FinalRequestMax  int
}

func resolveSummaryBudget(settings model.AppSettings, modelName string, stagePrompt string) summaryBudget {
	stageRequestMax, finalRequestMax, outputReserve := resolveOutputBudget(settings)
	chunkBudget := resolveChunkTokenBudget(modelName, stagePrompt, outputReserve)

	return summaryBudget{
		ChunkTokenBudget: chunkBudget,
		Parallelism:      resolveSummaryParallelism(settings.SummaryParallelism),
		StageRequestMax:  stageRequestMax,
		FinalRequestMax:  finalRequestMax,
	}
}

func resolveOutputBudget(settings model.AppSettings) (stageRequestMax int, finalRequestMax int, outputReserve int) {
	if settings.OpenAIOutputMode != model.OutputModeManual || settings.OpenAIMaxOutputToken <= 0 {
		return 0, 0, defaultStageOutputReserve
	}

	finalRequestMax = settings.OpenAIMaxOutputToken
	stageRequestMax = min(finalRequestMax, defaultStageOutputReserve)
	if stageRequestMax <= 0 {
		stageRequestMax = defaultStageOutputReserve
	}
	return stageRequestMax, finalRequestMax, stageRequestMax
}

func resolveChunkTokenBudget(modelName string, stagePrompt string, outputReserve int) int {
	contextWindow := approximateContextWindow(modelName)
	promptTokens := estimateTokens(stagePrompt)
	budget := int(float64(contextWindow)*defaultChunkContextRatio) - promptTokens - outputReserve - defaultChunkSafetyMargin
	if budget < minChunkTokenBudget {
		return minChunkTokenBudget
	}
	return budget
}

func approximateContextWindow(modelName string) int {
	modelName = strings.ToLower(strings.TrimSpace(modelName))

	switch {
	case strings.Contains(modelName, "gpt-5"):
		return 128000
	case strings.Contains(modelName, "gpt-4.1"):
		return 128000
	case strings.Contains(modelName, "gpt-4o"):
		return 128000
	default:
		return 32000
	}
}

func resolveSummaryParallelism(value int) int {
	if value <= 0 {
		return 2
	}
	return clampInt(value, 1, 6)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
