package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/localauth"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/scheduler"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
)

func TestFollowUpRouteRegistration(t *testing.T) {
	// Handler registers both method-qualified patterns and legacy prefix routes.
	r := &Router{}
	if r.Handler() == nil {
		t.Fatal("nil handler")
	}
}

func TestFollowUpRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct{ id, body string }{{"bad", `{"question":"q"}`}, {"-1", `{"question":"q"}`}, {"1", `{"question":" "}`}, {"1", `{"question":"q","extra":true}`}, {"1", strings.Repeat("x", 1024*1024+1)}} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		req.SetPathValue("summaryID", tc.id)
		w := httptest.NewRecorder()
		(&Router{}).handleSummaryFollowUp(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid input status %d", w.Code)
		}
	}
}

func followUpTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("TGTLDR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires dedicated TGTLDR_TEST_DATABASE_URL")
	}
	cfg := config.Config{DatabaseURL: dsn, MasterKey: make([]byte, 32)}
	admin, err := store.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("follow_up_%d", time.Now().UnixNano())
	if _, err = admin.Pool.Exec(t.Context(), "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Pool.Exec(context.Background(), "drop schema "+schema+" cascade"); admin.Close() })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	cfg.DatabaseURL = u.String()
	st, err := store.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err = store.RunMigrations(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestFollowUpIntegration(t *testing.T) {
	st := followUpTestStore(t)
	var requestedSource string
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.Messages) != 2 {
			t.Error("unexpected messages")
			return
		}
		requestedSource = payload.Messages[1].Content
		time.Sleep(150 * time.Millisecond)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"evidence answer"}}]}`)
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
	settings.DefaultTimezone = "UTC"
	if _, err = st.Settings.Save(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	_, err = st.Pool.Exec(t.Context(), `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode,filtered_keywords) overriding system value values(1,1,'test',true,'dashboard',ARRAY['excluded']);
 insert into summaries(id,chat_id,summary_date,status,content) overriding system value values(1,1,'2026-10-10','succeeded','saved summary'),(2,1,'2026-10-09','failed',''),(3,1,'2026-10-08','succeeded','empty day summary');
 insert into messages(chat_id,telegram_message_id,text_content,message_time) values(1,1,'evidence message','2026-10-10T08:00:00Z'),(1,2,'excluded message','2026-10-10T09:00:00Z'),(1,3,'next day message','2026-10-11T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	svc := summary.NewService(st, clock.System{}, 500*time.Millisecond)
	sched := scheduler.NewService(st, clock.System{}, svc, nil, nil)
	r := New(st, nil, sched, nil, nil, nil, "", 100*time.Millisecond, 500*time.Millisecond, "")
	session, err := localauth.NewService(st).SetupPassword(t.Context(), "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := r.Handler()
	for _, tc := range []struct {
		id     int
		status int
		source string
	}{{1, 200, "messages"}, {3, 200, "summary"}, {2, 400, ""}, {999, 404, ""}} {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/summaries/%d/follow-up", tc.id), strings.NewReader(`{"question":"what happened?"}`))
		w := httptest.NewRecorder()
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.SessionID})
		handler.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("id %d status %d: %s", tc.id, w.Code, w.Body.String())
		}
		if tc.status == 200 {
			var answer summary.FollowUpAnswer
			if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
				t.Fatal(err)
			}
			if answer.Answer != "evidence answer" || answer.Source != tc.source {
				t.Fatalf("answer %+v", answer)
			}
			if tc.id == 1 && (!strings.Contains(requestedSource, "evidence message") || strings.Contains(requestedSource, "excluded message") || strings.Contains(requestedSource, "next day message")) {
				t.Fatal("filter or date range not preserved")
			}
		}
	}
	item, err := st.Summaries.GetByID(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.BuildContextPreview(t.Context(), item)
	if err != nil || preview.MessageCount != 1 || len(preview.Chunks) != 1 || !strings.Contains(preview.Chunks[0].Content, "evidence message") || strings.Contains(preview.Chunks[0].Content, "excluded message") {
		t.Fatalf("preview changed: %+v %v", preview, err)
	}
}

func TestFollowUpRequestTimeout(t *testing.T) {
	r := New(nil, nil, nil, nil, nil, nil, "", 30*time.Millisecond, 100*time.Millisecond, "")
	for _, tc := range []struct {
		method, path string
		want         time.Duration
	}{
		{http.MethodPost, "/api/summaries/1/follow-up", 230 * time.Millisecond},
		{http.MethodPost, "/api/chats/1/follow-up", 230 * time.Millisecond},
		{http.MethodGet, "/api/summaries/1/follow-up", 30 * time.Millisecond},
		{http.MethodPost, "/api/summaries/1/follow-up/extra", 30 * time.Millisecond},
		{http.MethodGet, "/api/health", 30 * time.Millisecond},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if got := r.requestTimeout(req); got != tc.want {
			t.Fatalf("%s %s: got %v want %v", tc.method, tc.path, got, tc.want)
		}
		ctx, cancel := context.WithCancel(req.Context())
		timed, stop := context.WithTimeout(ctx, r.requestTimeout(req))
		cancel()
		if timed.Err() != context.Canceled {
			t.Fatal("caller cancellation not preserved")
		}
		stop()
	}
}
