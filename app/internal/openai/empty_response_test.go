package openai

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEmptyModelResponse 将普通与流式响应中的空正文识别为可重试错误。
func TestEmptyModelResponse(t *testing.T) {
	for _, stream := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if stream {
				_, _ = w.Write([]byte("data: {\"model\":\"terra\",\"choices\":[{\"delta\":{\"content\":\" \\n\"}}]}\n\ndata: [DONE]\n\n"))
				return
			}
			_, _ = w.Write([]byte(`{"model":"terra","choices":[{"message":{"content":" \n\t"}}]}`))
		}))
		client := New(Config{BaseURL: server.URL, Model: "luna", Stream: stream})
		response, err := client.Chat(t.Context(), ChatRequest{})
		server.Close()
		if !errors.Is(err, ErrEmptyResponse) || !IsRetryableError(err) || response.Model != "terra" {
			t.Fatalf("stream=%v response=%+v err=%v", stream, response, err)
		}
	}
}
