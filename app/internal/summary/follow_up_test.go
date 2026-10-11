package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

func TestFollowUpValidation(t *testing.T) {
	for _, q := range []string{"", "  ", strings.Repeat("字", 2001)} {
		if _, _, err := NormalizeFollowUp(q, nil); !errors.Is(err, ErrInvalidFollowUp) {
			t.Fatalf("invalid question accepted: %v", err)
		}
	}
	history := make([]FollowUpTurn, 8)
	for i := range history {
		history[i] = FollowUpTurn{Question: fmt.Sprintf("q%d", i), Answer: "answer"}
	}
	q, turns, err := NormalizeFollowUp(" question ", history)
	if err != nil || q != "question" || len(turns) != 6 || turns[0].Question != "q2" {
		t.Fatalf("normalization: %q %+v %v", q, turns, err)
	}
	if _, _, err := NormalizeFollowUp("ok", []FollowUpTurn{{Question: "q"}}); !errors.Is(err, ErrInvalidFollowUp) {
		t.Fatal("accepted empty history answer")
	}
	if _, _, err := NormalizeFollowUp(strings.Repeat("字", 2000), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := NormalizeFollowUp("next", []FollowUpTurn{{Question: "previous", Answer: strings.Repeat("a", 32001)}}); err != nil {
		t.Fatalf("rejected valid long model answer: %v", err)
	}

	if _, err := (&Service{}).AskFollowUp(t.Context(), model.Summary{Status: model.SummaryStatusFailed}, "q", nil); !errors.Is(err, ErrInvalidFollowUp) {
		t.Fatal("accepted failed summary")
	}
}

func TestFollowUpPrompts(t *testing.T) {
	for _, language := range []model.Language{model.LanguageEN, model.LanguageZhCN} {
		p := followUpSystemPrompt(language) + buildFollowUpPrompt(language, "2026-10-10", "summary", "source", "question", []FollowUpTurn{{Question: "old question", Answer: "old answer"}})
		for _, text := range []string{"source", "question", "old answer", "2026-10-10"} {
			if !strings.Contains(p, text) {
				t.Fatal("prompt missing", text)
			}
		}
		if language == model.LanguageEN {
			for _, r := range p {
				if unicode.Is(unicode.Han, r) {
					t.Fatal("English prompt has Chinese labels")
				}
			}
		} else if !strings.Contains(p, "摘要内容") || !strings.Contains(p, "对话历史") {
			t.Fatal("Chinese labels missing")
		}
	}
}

func TestFollowUpSourceBudget(t *testing.T) {
	settings := dailyTestSettings("unused")
	settings.OpenAIContextWindowTokens = 8000
	system := followUpSystemPrompt(settings.Language)
	budget := resolveSummaryBudget(settings, settings.OpenAIModel, system)
	item := model.Summary{SummaryDate: "2026-10-10", Content: "short summary"}
	for _, tc := range []struct{ transcript, source string }{{"short messages", "messages"}, {strings.Repeat("long transcript ", 10000), "summary"}, {"", "summary"}} {
		prompt, source, err := planFollowUp(settings.Language, item, tc.transcript, "q", nil, system, budget)
		if err != nil || source != tc.source || prompt == "" {
			t.Fatalf("source %s: %s %v", tc.source, source, err)
		}
	}
	item.Content = strings.Repeat("long summary ", 10000)
	if _, _, err := planFollowUp(settings.Language, item, "", "q", nil, system, budget); !errors.Is(err, ErrFollowUpContextLimit) {
		t.Fatal("oversized source accepted")
	}
	item.Content = "small"
	history := []FollowUpTurn{{Question: "q", Answer: strings.Repeat("history ", 10000)}}
	if _, _, err := planFollowUp(settings.Language, item, "messages", "q", history, system, budget); !errors.Is(err, ErrFollowUpContextLimit) {
		t.Fatal("history not counted in budget")
	}
}

func TestFollowUpModelRequest(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Model     string `json:"model"`
					Stream    bool   `json:"stream"`
					MaxOutput int    `json:"max_completion_tokens"`
					Messages  []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if payload.Stream != stream || payload.Model != "override-model" || payload.MaxOutput != 512 {
					t.Errorf("config ignored: %+v", payload)
				}
				if len(payload.Messages) != 2 || !strings.Contains(payload.Messages[1].Content, "old answer") {
					t.Error("missing history")
				}
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\ndata: [DONE]\n\n")
				} else {
					fmt.Fprint(w, `{"choices":[{"message":{"content":"answer"}}]}`)
				}
			}))
			defer server.Close()
			settings := dailyTestSettings(server.URL)
			settings.Language = model.LanguageEN
			settings.OpenAIOutputMode = model.OutputModeManual
			settings.OpenAIMaxOutputToken = 512
			if stream {
				settings.OpenAIRequestMode = model.OpenAIRequestModeStream
			}
			result, err := (&Service{openAITimeout: time.Second}).answerFollowUp(t.Context(), settings, model.Chat{ModelOverride: "override-model"}, model.Summary{Content: "summary", SummaryDate: "2026-10-10"}, "messages", "question", []FollowUpTurn{{Question: "old question", Answer: "old answer"}})
			if err != nil || result.Answer != "answer" || result.Source != "messages" {
				t.Fatalf("answer: %+v %v", result, err)
			}
		})
	}
}

func TestFollowUpProviderFallback(t *testing.T) {
	for _, tc := range []struct {
		name, transcript, content string
		firstStatus               int
		secondOverflow            bool
		wantSource                string
		wantCalls                 int
		wantContextError          bool
	}{
		{"messages rejected", "messages", "summary", 400, false, "summary", 2, false},
		{"both rejected", "messages", "summary", 400, true, "", 2, true},
		{"summary rejected", "", "summary", 400, false, "", 1, true},
		{"summary missing", "messages", "", 400, false, "", 1, true},
		{"summary too large", "messages", strings.Repeat("long ", 50000), 400, false, "", 1, true},
		{"unauthorized", "messages", "summary", 401, false, "", 1, false},
		{"rate limited", "messages", "summary", 429, false, "", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 || tc.secondOverflow {
					w.WriteHeader(tc.firstStatus)
					if tc.firstStatus == 400 {
						fmt.Fprint(w, `{"error":{"code":"context_length_exceeded","message":"context length exceeded"}}`)
					} else {
						fmt.Fprint(w, `{"error":{"message":"request rejected"}}`)
					}
					return
				}
				var payload struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if len(payload.Messages) != 2 || !strings.Contains(payload.Messages[1].Content, "Summary content") || !strings.Contains(payload.Messages[1].Content, "previous answer") {
					t.Error("retry lost summary or conversation")
				}
				fmt.Fprint(w, `{"choices":[{"message":{"content":"answer"}}]}`)
			}))
			defer server.Close()
			settings := dailyTestSettings(server.URL)
			settings.Language = model.LanguageEN
			settings.OpenAIContextWindowTokens = 8000
			result, err := (&Service{openAITimeout: time.Second}).answerFollowUp(t.Context(), settings, model.Chat{}, model.Summary{Content: tc.content}, tc.transcript, "q", []FollowUpTurn{{Question: "previous", Answer: "previous answer"}})
			if calls != tc.wantCalls || result.Source != tc.wantSource {
				t.Fatalf("calls=%d answer=%+v err=%v", calls, result, err)
			}
			if errors.Is(err, ErrFollowUpContextLimit) != tc.wantContextError {
				t.Fatalf("wrong context error: %v", err)
			}
			if tc.wantSource == "" && err == nil {
				t.Fatal("expected provider error")
			}
		})
	}
}

func TestFollowUpCancellation(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Consume the body so the server can detect a peer disconnect.
		var payload any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		close(started)
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(2 * time.Second):
			t.Error("model request was not cancelled")
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := (&Service{openAITimeout: time.Second}).answerFollowUp(ctx, dailyTestSettings(server.URL), model.Chat{}, model.Summary{Content: "summary"}, "messages", "q", nil)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("model never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not stop")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("model did not see cancellation")
	}
}

func TestFollowUpNaturalDayRange(t *testing.T) {
	for _, tc := range []struct {
		date  string
		hours time.Duration
	}{{"2026-03-08", 23}, {"2026-11-01", 25}, {"2026-10-10", 24}} {
		start, end, err := dayRange(tc.date, "America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		if end.Sub(start) != tc.hours*time.Hour {
			t.Fatalf("%s duration=%s", tc.date, end.Sub(start))
		}
		location, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		next := end.In(location)
		if next.Hour() != 0 || next.Minute() != 0 || next.Format("2006-01-02") != start.In(location).AddDate(0, 0, 1).Format("2006-01-02") {
			t.Fatalf("wrong local end %v", next)
		}
	}
}
