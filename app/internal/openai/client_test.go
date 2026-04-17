package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClientChatUsesMaxCompletionTokens(t *testing.T) {
	Convey("Chat 请求应该发送 max_completion_tokens", t, func() {
		var payload map[string]any
		var path string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
		}))
		defer server.Close()

		client := New(Config{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-5.4",
		})

		resp, err := client.Chat(context.Background(), ChatRequest{
			SystemPrompt: "system",
			UserPrompt:   "user",
			Temperature:  0.2,
			MaxOutput:    512,
		})

		So(err, ShouldBeNil)
		So(path, ShouldEqual, "/chat/completions")
		So(resp.Content, ShouldEqual, "ok")
		So(payload["max_completion_tokens"], ShouldEqual, float64(512))
		_, hasLegacyField := payload["max_tokens"]
		So(hasLegacyField, ShouldBeFalse)
	})
}

func TestClientChatOmitsMaxCompletionTokensWhenAuto(t *testing.T) {
	Convey("自动模式下不应该显式发送 max_completion_tokens", t, func() {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-5.4","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
		}))
		defer server.Close()

		client := New(Config{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-5.4",
		})

		_, err := client.Chat(context.Background(), ChatRequest{
			SystemPrompt: "system",
			UserPrompt:   "user",
			Temperature:  0.2,
			MaxOutput:    0,
		})

		So(err, ShouldBeNil)
		_, hasField := payload["max_completion_tokens"]
		So(hasField, ShouldBeFalse)
	})
}
