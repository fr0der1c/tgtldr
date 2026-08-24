package llmcontext

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/tiktoken-go/tokenizer"
)

const (
	DefaultContextWindow    = 32000
	DefaultOutputReserve    = 4096
	requestProtocolOverhead = 16
	minimumSafetyMargin     = 1024
)

// Counter 按模型使用对应的 tokenizer；未知模型使用偏保守的多语言估算。
type Counter struct {
	codec tokenizer.Codec
}

var (
	o200kOnce   sync.Once
	o200kCodec  tokenizer.Codec
	cl100kOnce  sync.Once
	cl100kCodec tokenizer.Codec
)

// NewCounter 为一次摘要任务创建可重复使用的 token 计数器。
func NewCounter(modelName string) Counter {
	encoding, ok := encodingForModel(modelName)
	if !ok {
		return Counter{}
	}
	codec := cachedCodec(encoding)
	if codec == nil {
		return Counter{}
	}
	return Counter{codec: codec}
}

// cachedCodec 复用只读 tokenizer 词表，避免并发摘要任务重复分配大词表。
func cachedCodec(encoding tokenizer.Encoding) tokenizer.Codec {
	switch encoding {
	case tokenizer.O200kBase:
		o200kOnce.Do(func() { o200kCodec, _ = tokenizer.Get(tokenizer.O200kBase) })
		return o200kCodec
	case tokenizer.Cl100kBase:
		cl100kOnce.Do(func() { cl100kCodec, _ = tokenizer.Get(tokenizer.Cl100kBase) })
		return cl100kCodec
	default:
		return nil
	}
}

// Count 返回文本的估算 token 数，计数失败时退回多语言估算。
func (c Counter) Count(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	if c.codec != nil {
		count, err := c.codec.Count(text)
		if err == nil {
			return count
		}
	}
	return estimateUnknownModelTokens(text)
}

// ResolveContextWindow 优先使用手动配置，再按已知模型解析上下文窗口。
func ResolveContextWindow(modelName string, configuredTokens int) int {
	if configuredTokens > 0 {
		return configuredTokens
	}

	modelName = normalizedModelName(modelName)
	switch {
	case strings.HasPrefix(modelName, "gpt-5.4-mini"), strings.HasPrefix(modelName, "gpt-5.4-nano"):
		return 400000
	case strings.HasPrefix(modelName, "gpt-5.4"):
		return 1050000
	case strings.HasPrefix(modelName, "gpt-5.2"):
		return 400000
	case strings.HasPrefix(modelName, "gpt-4.1"):
		return 1047576
	case strings.HasPrefix(modelName, "gpt-4o"):
		return 128000
	default:
		return DefaultContextWindow
	}
}

type RequestPlan struct {
	ContextWindow int
	PromptTokens  int
	InputTokens   int
	InputBudget   int
	OutputReserve int
	SafetyMargin  int
	Fits          bool
}

// PlanRequest 计算一次模型调用的输入容量，并判断完整输入是否可直接发送。
func PlanRequest(counter Counter, contextWindow int, systemPrompt, userPrompt string, outputReserve int) RequestPlan {
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}
	if outputReserve <= 0 {
		outputReserve = DefaultOutputReserve
	}

	promptTokens := counter.Count(systemPrompt) + requestProtocolOverhead
	inputTokens := counter.Count(userPrompt)
	safetyMargin := max(minimumSafetyMargin, contextWindow/20)
	inputBudget := contextWindow - promptTokens - outputReserve - safetyMargin
	if inputBudget < 0 {
		inputBudget = 0
	}

	return RequestPlan{
		ContextWindow: contextWindow,
		PromptTokens:  promptTokens,
		InputTokens:   inputTokens,
		InputBudget:   inputBudget,
		OutputReserve: outputReserve,
		SafetyMargin:  safetyMargin,
		Fits:          inputTokens <= inputBudget,
	}
}

// encodingForModel 将已知模型族映射到其文本 tokenizer。
func encodingForModel(modelName string) (tokenizer.Encoding, bool) {
	modelName = normalizedModelName(modelName)
	switch {
	case strings.HasPrefix(modelName, "gpt-5"),
		strings.HasPrefix(modelName, "gpt-4.1"),
		strings.HasPrefix(modelName, "gpt-4o"),
		strings.HasPrefix(modelName, "o1"),
		strings.HasPrefix(modelName, "o3"),
		strings.HasPrefix(modelName, "o4"):
		return tokenizer.O200kBase, true
	case strings.HasPrefix(modelName, "gpt-4"), strings.HasPrefix(modelName, "gpt-3.5"):
		return tokenizer.Cl100kBase, true
	default:
		return "", false
	}
}

func normalizedModelName(modelName string) string {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if separator := strings.LastIndex(modelName, "/"); separator >= 0 {
		modelName = modelName[separator+1:]
	}
	return modelName
}

// estimateUnknownModelTokens 分别估算 ASCII 和非 ASCII 文本，避免低估中文输入。
func estimateUnknownModelTokens(text string) int {
	asciiRunes := 0
	nonASCIIRunes := 0
	for _, value := range text {
		if value <= utf8.RuneSelf {
			asciiRunes++
			continue
		}
		nonASCIIRunes++
	}
	return (asciiRunes+3)/4 + nonASCIIRunes
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
