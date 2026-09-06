package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
	stream     bool
}

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
	Stream  bool
}

type ChatRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	MaxOutput    int
}

type ChatResponse struct {
	Content string
	Model   string
}

type chatCompletionRequest struct {
	Model               string        `json:"model"`
	Temperature         float64       `json:"temperature,omitempty"`
	MaxCompletionTokens int           `json:"max_completion_tokens,omitempty"`
	Stream              bool          `json:"stream,omitempty"`
	Messages            []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type chatCompletionStreamResponse struct {
	Model string `json:"model"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	Choices []struct {
		Delta chatMessage `json:"delta"`
	} `json:"choices"`
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}

	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		stream:     cfg.Stream,
	}
}

// Chat 将没有可用正文的响应视为临时失败，交由摘要任务的有限重试策略处理。
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	var response ChatResponse
	var err error
	if c.stream {
		response, err = c.chatStream(ctx, req)
	} else {
		response, err = c.chatNonStream(ctx, req)
	}
	if err == nil && strings.TrimSpace(response.Content) == "" {
		err = ErrEmptyResponse
	}
	return response, err
}

func (c *Client) chatNonStream(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	payload := chatCompletionRequest{
		Model:               c.model,
		Temperature:         req.Temperature,
		MaxCompletionTokens: req.MaxOutput,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("build chat request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("request chat completion: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("read chat completion: %w", err)
	}
	if resp.StatusCode >= 300 {
		return ChatResponse{}, decodeAPIError(resp.StatusCode, respBody)
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return ChatResponse{}, fmt.Errorf("decode chat completion: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("openai returned no choices")
	}
	return ChatResponse{
		Content: decoded.Choices[0].Message.Content,
		Model:   decoded.Model,
	}, nil
}

func (c *Client) chatStream(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	payload := chatCompletionRequest{
		Model:               c.model,
		Temperature:         req.Temperature,
		MaxCompletionTokens: req.MaxOutput,
		Stream:              true,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("build chat request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("request chat completion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return ChatResponse{}, fmt.Errorf("read chat completion: %w", err)
		}
		return ChatResponse{}, decodeAPIError(resp.StatusCode, respBody)
	}

	return c.readChatStream(resp.Body)
}

func (c *Client) readChatStream(body io.Reader) (ChatResponse, error) {
	reader := bufio.NewReader(body)
	var content strings.Builder
	model := c.model
	done := false

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			lineDone, lineErr := readChatStreamLine(strings.TrimSpace(line), &content, &model)
			if lineErr != nil {
				return ChatResponse{}, lineErr
			}
			done = done || lineDone
		}
		if done {
			return ChatResponse{Content: content.String(), Model: model}, nil
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return ChatResponse{}, fmt.Errorf("openai stream ended before done")
		}
		return ChatResponse{}, fmt.Errorf("read chat completion stream: %w", err)
	}
}

func readChatStreamLine(line string, content *strings.Builder, model *string) (bool, error) {
	if line == "" || strings.HasPrefix(line, ":") {
		return false, nil
	}
	if !strings.HasPrefix(line, "data:") {
		return false, nil
	}

	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "[DONE]" {
		return true, nil
	}

	var decoded chatCompletionStreamResponse
	if err := json.Unmarshal([]byte(data), &decoded); err != nil {
		return false, fmt.Errorf("decode chat completion stream: %w", err)
	}
	if decoded.Error != nil {
		message := strings.TrimSpace(decoded.Error.Message)
		if message == "" {
			message = data
		}
		return false, &APIError{
			Code:     decoded.Error.Code,
			Type:     decoded.Error.Type,
			Message:  message,
			Response: compactErrorResponse(data),
		}
	}
	if decoded.Model != "" {
		*model = decoded.Model
	}
	for _, choice := range decoded.Choices {
		content.WriteString(choice.Delta.Content)
	}
	return false, nil
}
