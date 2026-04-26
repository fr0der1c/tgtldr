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
)

type Service struct {
	client *http.Client
}

func New() *Service {
	return &Service{client: &http.Client{Timeout: 20 * time.Second}}
}

func (s *Service) SendMessage(ctx context.Context, token, chatID, text string) error {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("missing bot token or chat id")
	}

	formatted := formatTelegramMessage(text)
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
		"https://api.telegram.org/bot"+token+"/sendMessage",
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
