package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

const maxErrorResponseLength = 2000

type APIError struct {
	StatusCode int
	Code       string
	Type       string
	Message    string
	Response   string
}

// IsRetryableError 仅重试限频、超时、网络错误和服务端临时故障。
func IsRetryableError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return true
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return true
	}
	if IsContextLimitError(apiErr) {
		return false
	}
	if apiErr.StatusCode == 408 || apiErr.StatusCode == 409 || apiErr.StatusCode == 429 || apiErr.StatusCode >= 500 {
		return true
	}
	if apiErr.StatusCode >= 400 {
		return false
	}
	signal := strings.ToLower(apiErr.Code + " " + apiErr.Message + " " + apiErr.Response)
	return strings.Contains(signal, "rate_limit") ||
		strings.Contains(signal, "rate limit") ||
		strings.Contains(signal, "timeout") ||
		strings.Contains(signal, "temporarily unavailable") ||
		strings.Contains(signal, "server_error")
}

// Error 优先使用结构化消息，并保留上游 HTTP 状态用于排查。
func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Response)
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("openai status %d: %s", e.StatusCode, message)
	}
	return "openai stream error: " + message
}

// IsContextLimitError 识别可通过缩小输入解决的上下文或请求体超限错误。
func IsContextLimitError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode == 413 {
		return true
	}

	code := strings.ToLower(strings.TrimSpace(apiErr.Code))
	if code == "context_length_exceeded" || code == "max_tokens_exceeded" || code == "request_too_large" {
		return true
	}
	message := strings.ToLower(apiErr.Message + " " + apiErr.Response)
	patterns := []string{
		"context length",
		"context window",
		"maximum context",
		"too many tokens",
		"request too large",
		"input is too long",
		"prompt is too long",
	}
	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

// decodeAPIError 解析 OpenAI-compatible 错误，同时保留长度受限的响应摘要。
func decodeAPIError(statusCode int, body []byte) *APIError {
	response := compactErrorResponse(string(body))
	payload := struct {
		Error struct {
			Code    string `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}{}
	_ = json.Unmarshal(body, &payload)
	return &APIError{
		StatusCode: statusCode,
		Code:       payload.Error.Code,
		Type:       payload.Error.Type,
		Message:    payload.Error.Message,
		Response:   response,
	}
}

// compactErrorResponse 按 Unicode 字符截断上游响应，避免日志和任务记录过大。
func compactErrorResponse(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxErrorResponseLength {
		return value
	}
	return string(runes[:maxErrorResponseLength]) + "…"
}
