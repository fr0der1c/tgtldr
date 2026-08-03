package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

func TestParseReadAPIChatPath(t *testing.T) {
	chatID, resource, ok := parseReadAPIChatPath("/api/v1/chats/12/messages")
	if !ok || chatID != 12 || resource != "messages" {
		t.Fatalf("unexpected path result: chatID=%d resource=%q ok=%v", chatID, resource, ok)
	}
	if _, _, ok := parseReadAPIChatPath("/api/v1/chats/12/messages/extra"); ok {
		t.Fatal("expected path with extra segment to be rejected")
	}
}

func TestReadAPIMessageOmitsInternalFields(t *testing.T) {
	payload, err := json.Marshal(toReadAPIMessage(model.Message{
		ID:                99,
		ChatID:            1,
		TelegramMessageID: 123,
		TelegramSenderID:  8619940043,
		TextContent:       "hello",
		RawJSON:           `{"secret":"value"}`,
	}))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	got := string(payload)
	if got == "" || !containsJSONField(got, "telegramMessageId") {
		t.Fatalf("missing public message fields: %s", got)
	}
	if containsJSONField(got, "id") || containsJSONField(got, "chatId") || containsJSONField(got, "rawJson") {
		t.Fatalf("internal field leaked: %s", got)
	}
}

func containsJSONField(payload string, field string) bool {
	var decoded map[string]any
	if json.Unmarshal([]byte(payload), &decoded) != nil {
		return false
	}
	_, ok := decoded[field]
	return ok
}

func TestReadAPIAuthenticationMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	t.Run("disabled without configured token", func(t *testing.T) {
		router := &Router{timeout: time.Second}
		recorder := httptest.NewRecorder()
		router.withMiddleware(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("rejects missing token", func(t *testing.T) {
		router := &Router{readAPIToken: "secret", timeout: time.Second}
		recorder := httptest.NewRecorder()
		router.withMiddleware(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	t.Run("accepts matching token", func(t *testing.T) {
		router := &Router{readAPIToken: "secret", timeout: time.Second}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil)
		req.Header.Set("Authorization", "Bearer secret")
		recorder := httptest.NewRecorder()
		router.withMiddleware(next).ServeHTTP(recorder, req)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
	})

	if isReadAPIPath("/api/settings") {
		t.Fatal("read API token must not authorize existing API paths")
	}
}

func TestReadAPIRequestAuthorized(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/chats", nil)
	req.Header.Set("Authorization", "Bearer correct-token")
	if !readAPIRequestAuthorized(req, "correct-token") {
		t.Fatal("expected matching bearer token to be authorized")
	}

	req.Header.Set("Authorization", "Bearer wrong-token")
	if readAPIRequestAuthorized(req, "correct-token") {
		t.Fatal("expected mismatched bearer token to be rejected")
	}

	req.Header.Del("Authorization")
	if readAPIRequestAuthorized(req, "correct-token") {
		t.Fatal("expected missing bearer token to be rejected")
	}
}

func TestMessageCursorRoundTrip(t *testing.T) {
	want := messageCursor{
		MessageTime:       time.Date(2026, 8, 3, 10, 20, 30, 123456000, time.UTC),
		TelegramMessageID: 456,
	}

	raw := encodeMessageCursor(want)
	got, err := decodeMessageCursor(raw)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if !got.MessageTime.Equal(want.MessageTime) || got.TelegramMessageID != want.TelegramMessageID {
		t.Fatalf("cursor mismatch: got %#v want %#v", got, want)
	}

	if _, err := decodeMessageCursor("not-a-cursor"); err == nil {
		t.Fatal("expected invalid cursor to fail")
	}
}

func TestParseMessageListQuery(t *testing.T) {
	values := url.Values{
		"senderId": {"8619940043"},
		"from":     {"2026-07-01T00:00:00+08:00"},
		"to":       {"2026-08-01T00:00:00+08:00"},
		"limit":    {"999"},
	}

	got, err := parseMessageListQuery(values)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if got.SenderID == nil || *got.SenderID != 8619940043 {
		t.Fatalf("sender id = %v", got.SenderID)
	}
	if got.Limit != maxReadAPILimit {
		t.Fatalf("limit = %d, want %d", got.Limit, maxReadAPILimit)
	}
	if got.From == nil || got.To == nil || !got.From.Before(*got.To) {
		t.Fatalf("invalid parsed range: %#v", got)
	}

	groupQuery, err := parseMessageListQuery(url.Values{})
	if err != nil {
		t.Fatalf("parse whole-chat query: %v", err)
	}
	if groupQuery.SenderID != nil {
		t.Fatalf("whole-chat query sender id = %v, want nil", groupQuery.SenderID)
	}
	if _, err := parseMessageListQuery(url.Values{"senderId": {"0"}}); err == nil {
		t.Fatal("expected invalid senderId to fail")
	}
	if _, err := parseMessageListQuery(url.Values{
		"senderId": {"1"},
		"from":     {"2026-08-01T00:00:00Z"},
		"to":       {"2026-07-01T00:00:00Z"},
	}); err == nil {
		t.Fatal("expected reversed date range to fail")
	}
}

func TestParseParticipantQuery(t *testing.T) {
	query, limit, err := parseParticipantQuery(url.Values{
		"query": {" @sssssssssshai "},
		"limit": {"200"},
	})
	if err != nil {
		t.Fatalf("parse participant query: %v", err)
	}
	if query != "sssssssssshai" {
		t.Fatalf("query = %q", query)
	}
	if limit != maxParticipantLimit {
		t.Fatalf("limit = %d, want %d", limit, maxParticipantLimit)
	}

	if _, _, err := parseParticipantQuery(url.Values{}); err == nil {
		t.Fatal("expected missing query to fail")
	}
}
