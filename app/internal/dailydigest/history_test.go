package dailydigest

import (
	"strings"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/rollup"
)

// TestBoundedHistory 验证预算优先保留近期内容，截断可见且旧来源编号不会混入当天引用。
func TestBoundedHistory(t *testing.T) {
	counter := llmcontext.NewCounter("custom-model")
	history := []model.DailyDigest{
		{SummaryDate: "2026-10-04", Content: strings.Repeat("近期讨论 [S009]\n", 100)},
		{SummaryDate: "2026-10-03", Content: "较早讨论"},
	}
	blocks := boundedHistory(history, counter, 400, "[截断]")
	input := strings.Join(blocks, digestContextSeparator)
	if len(blocks) != 1 || !strings.HasPrefix(input, "2026-10-04") || !strings.Contains(input, "[截断]") {
		t.Fatalf("unexpected excerpt: %s", input)
	}
	if strings.Contains(input, "[S009]") || strings.Contains(input, "较早讨论") || counter.Count(input) > 400 {
		t.Fatalf("invalid history budget or sources: %s", input)
	}
	blocks = boundedHistory(history, counter, 0, "[截断]")
	if len(blocks) != 0 {
		t.Fatal("zero budget must omit history")
	}
}

// TestHistoryKeepsCurrentInput 验证历史在剩余容量不足时先被缩减，不额外触发当天分块。
func TestHistoryKeepsCurrentInput(t *testing.T) {
	for _, language := range []model.Language{model.LanguageZhCN, model.LanguageEN} {
		t.Run(string(language), func(t *testing.T) {
			settings := digestTestSettings("")
			settings.Language = language
			settings.OpenAIModel = "custom-model"
			settings.OpenAIContextWindowTokens = 10000
			settings.OpenAIOutputMode = model.OutputModeManual
			settings.OpenAIMaxOutputToken = 500
			counter := llmcontext.NewCounter(settings.OpenAIModel)
			prompt := buildFinalPrompt(language)
			plan := llmcontext.PlanRequest(counter, 10000, prompt, "", 500)
			sources := []rollup.Source{{Header: "[S001]\n", Content: strings.Repeat("证", plan.InputBudget-100)}}
			history := []model.DailyDigest{{SummaryDate: "2026-10-04", Content: strings.Repeat("旧信息", 5000)}}
			context := buildFinalContext(settings, "2026-10-05", history, sources)
			userInput := sources[0].Header + sources[0].Content + digestContextSeparator + context
			if !llmcontext.PlanRequest(counter, 10000, prompt, userInput, 500).Fits {
				t.Fatalf("history displaced current input: %s", context)
			}
			if strings.Contains(context, "旧信息") || !strings.Contains(context, "2026-10-05") {
				t.Fatalf("expected history omission with target date: %s", context)
			}
		})
	}
}

// TestOmittedHistoryFitsExactBudget 验证当天证据占满剩余容量时，省略历史不会增加说明文本。
func TestOmittedHistoryFitsExactBudget(t *testing.T) {
	settings := digestTestSettings("")
	settings.OpenAIModel = "custom-model"
	settings.OpenAIContextWindowTokens = 10000
	settings.OpenAIOutputMode = model.OutputModeManual
	settings.OpenAIMaxOutputToken = 500
	counter := llmcontext.NewCounter(settings.OpenAIModel)
	prompt := buildFinalPrompt(settings.Language)
	plan := llmcontext.PlanRequest(counter, 10000, prompt, "", 500)
	dateHeader := "总览日期：2026-10-05"
	source := rollup.Source{Header: "[S001]\n"}
	source.Content = strings.Repeat("证", plan.InputBudget-counter.Count(source.Header+digestContextSeparator+dateHeader))
	for _, history := range [][]model.DailyDigest{nil, {{SummaryDate: "2026-10-04", Content: "此前讨论"}}} {
		context := buildFinalContext(settings, "2026-10-05", history, []rollup.Source{source})
		input := source.Header + source.Content + digestContextSeparator + context
		if !llmcontext.PlanRequest(counter, 10000, prompt, input, 500).Fits {
			t.Fatalf("omitted history displaced current evidence: %s", context)
		}
	}
}

// TestHistoryContextLimit 验证当天需要分块时仍有历史对照，并且历史受总预算限制。
func TestHistoryContextLimit(t *testing.T) {
	settings := digestTestSettings("")
	settings.OpenAIModel = "custom-model"
	sources := []rollup.Source{{Header: "[S001]\n", Content: strings.Repeat("当天信息", 30000)}}
	history := []model.DailyDigest{{SummaryDate: "2026-10-04", Content: strings.Repeat("旧信息", 5000)}}
	context := buildFinalContext(settings, "2026-10-05", history, sources)
	if !strings.Contains(context, "2026-10-04") || !strings.Contains(context, "截断") {
		t.Fatalf("missing bounded comparison context: %s", context)
	}
	if llmcontext.NewCounter(settings.OpenAIModel).Count(context) > maximumHistoryTokens+200 {
		t.Fatal("history exceeded its bounded budget")
	}
}
