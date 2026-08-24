package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

type Service struct {
	client     *http.Client
	apiBaseURL string
}

func New() *Service {
	return newService("https://api.telegram.org")
}

// newService 构造指定 Bot API 根地址的服务，New 固定使用 Telegram 官方地址。
func newService(apiBaseURL string) *Service {
	return &Service{
		client:     &http.Client{Timeout: 20 * time.Second},
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
	}
}

func (s *Service) SendMessage(ctx context.Context, token, chatID, text string) error {
	return s.SendMessageWithLanguage(ctx, token, chatID, text, model.LanguageZhCN)
}

func (s *Service) SendMessageWithLanguage(ctx context.Context, token, chatID, text string, language model.Language) error {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("missing bot token or chat id")
	}

	formatted := formatTelegramMessage(text, language)
	return s.sendHTML(ctx, token, chatID, formatted)
}

// SendLongMessage 将完整正文按 Telegram 限制拆分后顺序发送。
func (s *Service) SendLongMessage(ctx context.Context, token, chatID, text string) error {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("missing bot token or chat id")
	}
	messages := formatTelegramMessages(text)
	for index, message := range messages {
		if err := s.sendHTML(ctx, token, chatID, message); err != nil {
			return fmt.Errorf("send bot message part %d of %d: %w", index+1, len(messages), err)
		}
	}
	return nil
}

// sendHTML 发送一条已经满足 Telegram 长度与 HTML 子集约束的消息。
func (s *Service) sendHTML(ctx context.Context, token, chatID, formatted string) error {
	payload, err := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"text":                     formatted,
		"parse_mode":               "HTML",
		"disable_web_page_preview": false,
	})
	if err != nil {
		return fmt.Errorf("marshal bot payload: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.apiBaseURL+"/bot"+token+"/sendMessage",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("build bot request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send bot message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read bot response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bot status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
