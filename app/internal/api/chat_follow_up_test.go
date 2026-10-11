package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/localauth"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/scheduler"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
)

func TestChatFollowUpIntegration(t *testing.T) {
	st := followUpTestStore(t)
	var prompt string
	overflowOnce := true
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Messages) != 2 {
			t.Error("unexpected request", err)
			return
		}
		prompt = payload.Messages[1].Content
		overflow := strings.Contains(prompt, "always-overflow")
		if strings.Contains(prompt, "c3 day") && overflowOnce {
			overflowOnce, overflow = false, true
		}
		if overflow {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"code":"context_length_exceeded","message":"context length exceeded"}}`)
			return
		}
		time.Sleep(150 * time.Millisecond)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"chat answer"}}]}`)
	}))
	defer modelServer.Close()
	settings, err := st.Settings.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	settings.OpenAIBaseURL = modelServer.URL
	settings.OpenAIAPIKey = "test"
	settings.OpenAIModel = "test-model"
	settings.OpenAIRequestMode = model.OpenAIRequestModeNonStream
	settings.Language = model.LanguageEN
	if _, err = st.Settings.Save(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	_, err = st.Pool.Exec(t.Context(), `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode) overriding system value values(1,1,'test',true,'dashboard'),(2,2,'empty',true,'dashboard'),(3,3,'retry',true,'dashboard'),(4,4,'overflow',true,'dashboard');
 insert into summaries(chat_id,summary_date,status,content) values
 (1,'2026-10-11','succeeded','later summary'),(1,'2026-10-10','succeeded','tenth summary'),(1,'2026-10-09','failed','failed summary'),
 (1,'2026-10-08','succeeded','eighth summary'),(1,'2026-10-07','succeeded','  '),
 (3,'2026-10-01','succeeded','c3 day 1'),(3,'2026-10-02','succeeded','c3 day 2'),(3,'2026-10-03','succeeded','c3 day 3'),(3,'2026-10-04','succeeded','c3 day 4'),
 (4,'2026-10-01','succeeded','always-overflow')`)
	if err != nil {
		t.Fatal(err)
	}
	svc := summary.NewService(st, clock.System{}, 500*time.Millisecond)
	r := New(st, nil, scheduler.NewService(st, clock.System{}, svc, nil, nil), nil, nil, nil, "", 100*time.Millisecond, 500*time.Millisecond, "")
	session, err := localauth.NewService(st).SetupPassword(t.Context(), "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := r.Handler()
	for _, tc := range []struct {
		chat   int
		body   string
		status int
	}{
		{1, `{"toDate":"2026-10-10","question":"what changed?"}`, 200},
		{2, `{"toDate":"2026-10-10","question":"what changed?"}`, 400},
		{999, `{"toDate":"2026-10-10","question":"what changed?"}`, 404},
		{1, `{"toDate":"10/10","question":"what changed?"}`, 400},
		{1, `{"toDate":"2026-10-10","question":" "}`, 400},
		{4, `{"toDate":"2026-10-10","question":"what changed?"}`, 400},
	} {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/chats/%d/follow-up", tc.chat), strings.NewReader(tc.body))
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.SessionID})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("chat %d %s: status %d %s", tc.chat, tc.body, w.Code, w.Body.String())
		}
		if tc.chat == 4 && !strings.Contains(w.Body.String(), "follow_up_context_limit") {
			t.Fatalf("context limit code missing: %s", w.Body.String())
		}
		if tc.status != 200 {
			continue
		}
		var answer summary.ChatFollowUpAnswer
		if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
			t.Fatal(err)
		}
		if answer.Answer != "chat answer" || answer.FromDate != "2026-10-08" || answer.ToDate != "2026-10-10" || answer.SummaryCount != 2 {
			t.Fatalf("answer %+v", answer)
		}
		if strings.Contains(prompt, "later summary") || strings.Contains(prompt, "failed summary") ||
			strings.Index(prompt, "eighth summary") > strings.Index(prompt, "tenth summary") {
			t.Fatalf("prompt sources: %s", prompt)
		}
	}

	// 服务商报告上下文超限时，只保留较近的一半摘要重试一次。
	req := httptest.NewRequest(http.MethodPost, "/api/chats/3/follow-up", strings.NewReader(`{"toDate":"2026-10-10","question":"what changed?"}`))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.SessionID})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var retried summary.ChatFollowUpAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &retried); w.Code != 200 || err != nil {
		t.Fatalf("retry status %d: %s", w.Code, w.Body.String())
	}
	if retried.SummaryCount != 2 || retried.FromDate != "2026-10-03" || retried.ToDate != "2026-10-04" || strings.Contains(prompt, "c3 day 2") {
		t.Fatalf("retry answer %+v", retried)
	}
}
