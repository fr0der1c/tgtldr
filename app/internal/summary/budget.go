package summary

import (
	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	"github.com/fr0der1c/tgtldr/app/internal/model"
)

const (
	defaultStageOutputReserve = 1200
	minimumChunkTokenBudget   = 512
)

type summaryBudget struct {
	ChunkTokenBudget int
	ContextWindow    int
	Counter          llmcontext.Counter
	Parallelism      int
	StageRequestMax  int
	FinalRequestMax  int
	StageReserve     int
	FinalReserve     int
}

func resolveSummaryBudget(settings model.AppSettings, modelName string, stagePrompt string) summaryBudget {
	stageRequestMax, finalRequestMax, stageReserve, finalReserve := resolveOutputBudget(settings)
	configuredContext := 0
	if settings.OpenAIContextWindowMode == model.ContextWindowModeManual {
		configuredContext = settings.OpenAIContextWindowTokens
	}
	contextWindow := llmcontext.ResolveContextWindow(modelName, configuredContext)
	counter := llmcontext.NewCounter(modelName)
	stagePlan := llmcontext.PlanRequest(counter, contextWindow, stagePrompt, "", stageReserve)
	chunkBudget := stagePlan.InputBudget
	if chunkBudget < minimumChunkTokenBudget {
		chunkBudget = minimumChunkTokenBudget
	}

	return summaryBudget{
		ChunkTokenBudget: chunkBudget,
		ContextWindow:    contextWindow,
		Counter:          counter,
		Parallelism:      resolveSummaryParallelism(settings.SummaryParallelism),
		StageRequestMax:  stageRequestMax,
		FinalRequestMax:  finalRequestMax,
		StageReserve:     stageReserve,
		FinalReserve:     finalReserve,
	}
}

func resolveOutputBudget(settings model.AppSettings) (stageRequestMax int, finalRequestMax int, stageReserve int, finalReserve int) {
	if settings.OpenAIOutputMode != model.OutputModeManual || settings.OpenAIMaxOutputToken <= 0 {
		return 0, 0, defaultStageOutputReserve, llmcontext.DefaultOutputReserve
	}

	finalRequestMax = settings.OpenAIMaxOutputToken
	stageRequestMax = min(finalRequestMax, defaultStageOutputReserve)
	if stageRequestMax <= 0 {
		stageRequestMax = defaultStageOutputReserve
	}
	return stageRequestMax, finalRequestMax, stageRequestMax, finalRequestMax
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
