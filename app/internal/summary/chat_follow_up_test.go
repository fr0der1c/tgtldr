package summary

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

func TestChatFollowUpPrompts(t *testing.T) {
	summaries := []model.Summary{{SummaryDate: "2026-10-10", Content: "newer"}, {SummaryDate: "2026-10-09", Content: "older"}}
	history := []FollowUpTurn{{Question: "old question", Answer: "old answer"}}
	for _, language := range []model.Language{model.LanguageEN, model.LanguageZhCN} {
		p := chatFollowUpSystemPrompt(language) + buildChatFollowUpPrompt(language, summaries, "question", history)
		for _, text := range []string{"newer", "older", "2026-10-09", "old answer", "question"} {
			if !strings.Contains(p, text) {
				t.Fatal("prompt missing", text)
			}
		}
		if strings.Index(p, "older") > strings.Index(p, "newer") {
			t.Fatal("summaries are not in chronological order")
		}
		if language == model.LanguageEN {
			for _, r := range p {
				if unicode.Is(unicode.Han, r) {
					t.Fatal("English prompt has Chinese labels")
				}
			}
		}
	}
}

func TestFittingChatSummaryCount(t *testing.T) {
	settings := dailyTestSettings("unused")
	settings.OpenAIContextWindowTokens = 8000
	system := chatFollowUpSystemPrompt(settings.Language)
	budget := resolveSummaryBudget(settings, settings.OpenAIModel, system)
	small := make([]model.Summary, 5)
	large := make([]model.Summary, 40)
	for i := range large {
		large[i] = model.Summary{SummaryDate: fmt.Sprintf("2026-09-%02d", 30-i%30), Content: strings.Repeat("detail ", 400)}
		if i < len(small) {
			small[i] = model.Summary{SummaryDate: large[i].SummaryDate, Content: "short"}
		}
	}
	if got := fittingChatSummaryCount(settings.Language, small, "q", nil, system, budget); got != len(small) {
		t.Fatalf("small summaries: %d", got)
	}
	got := fittingChatSummaryCount(settings.Language, large, "q", nil, system, budget)
	if got == 0 || got >= len(large) {
		t.Fatalf("large summaries: %d", got)
	}
	if fittingChatSummaryCount(settings.Language, large[:1], strings.Repeat("q ", 1), []FollowUpTurn{{Question: "q", Answer: strings.Repeat("history ", 10000)}}, system, budget) != 0 {
		t.Fatal("history not counted in budget")
	}
}
