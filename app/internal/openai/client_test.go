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
		_, hasStreamField := payload["stream"]
		So(hasStreamField, ShouldBeFalse)
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

func TestClientChatStream(t *testing.T) {
	Convey("流式 Chat 请求应该发送 stream 并拼接 delta 内容", t, func() {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("\n"))
			_, _ = w.Write([]byte(": keep-alive\n\n"))
			_, _ = w.Write([]byte(`data: {"model":"gpt-5.4","choices":[{"delta":{"content":"hel"}}]}` + "\n\n"))
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"lo"}}]}` + "\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}))
		defer server.Close()

		client := New(Config{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-5.4",
			Stream:  true,
		})

		resp, err := client.Chat(context.Background(), ChatRequest{
			SystemPrompt: "system",
			UserPrompt:   "user",
			Temperature:  0.2,
			MaxOutput:    512,
		})

		So(err, ShouldBeNil)
		So(resp.Content, ShouldEqual, "hello")
		So(resp.Model, ShouldEqual, "gpt-5.4")
		So(payload["stream"], ShouldEqual, true)
		So(payload["max_completion_tokens"], ShouldEqual, float64(512))
	})

	Convey("流式上游错误会读取响应正文", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}))
		defer server.Close()

		client := New(Config{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-5.4",
			Stream:  true,
		})

		_, err := client.Chat(context.Background(), ChatRequest{
			SystemPrompt: "system",
			UserPrompt:   "user",
		})

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "openai status 502")
		So(err.Error(), ShouldContainSubstring, "bad gateway")
	})

	Convey("流式响应没有 DONE 时应该报错", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n"))
		}))
		defer server.Close()

		client := New(Config{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-5.4",
			Stream:  true,
		})

		_, err := client.Chat(context.Background(), ChatRequest{
			SystemPrompt: "system",
			UserPrompt:   "user",
		})

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "openai stream ended before done")
	})

	Convey("流式错误事件应该返回错误", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"error":{"message":"rate limited"}}` + "\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}))
		defer server.Close()

		client := New(Config{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-5.4",
			Stream:  true,
		})

		_, err := client.Chat(context.Background(), ChatRequest{
			SystemPrompt: "system",
			UserPrompt:   "user",
		})

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "openai stream error: rate limited")
	})
}
