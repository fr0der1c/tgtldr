package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type APIError struct {
	StatusCode  int
	Description string
}

func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Description)
	if message != "" {
		return message
	}
	return fmt.Sprintf("bot status %d", e.StatusCode)
}

type TargetChatCandidate struct {
	ChatID   string `json:"chatId"`
	ChatType string `json:"chatType"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
}

type botAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

type botUpdate struct {
	UpdateID int64       `json:"update_id"`
	Message  *botMessage `json:"message"`
}

type botMessage struct {
	Date int64    `json:"date"`
	From *botUser `json:"from"`
	Chat botChat  `json:"chat"`
}

type botUser struct {
	ID int64 `json:"id"`
}

type botChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type targetChatCandidateRecord struct {
	TargetChatCandidate
	messageDate int64
	updateID    int64
}

func (s *Service) ResolveTargetChats(
	ctx context.Context,
	token string,
	telegramUserID int64,
) ([]TargetChatCandidate, error) {
	updates, err := s.getUpdates(ctx, token)
	if err != nil {
		return nil, err
	}
	return matchTargetChatCandidates(updates, telegramUserID), nil
}

func (s *Service) getUpdates(ctx context.Context, token string) ([]botUpdate, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, fmt.Errorf("missing bot token")
	}

	endpoint, err := url.Parse("https://api.telegram.org/bot" + trimmed + "/getUpdates")
	if err != nil {
		return nil, fmt.Errorf("build getUpdates url: %w", err)
	}

	query := endpoint.Query()
	query.Set("allowed_updates", `["message"]`)
	query.Set("limit", "100")
	query.Set("timeout", "5")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build getUpdates request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get bot updates: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read getUpdates response: %w", err)
	}

	var payload botAPIResponse[[]botUpdate]
	if err := json.Unmarshal(body, &payload); err != nil {
		if resp.StatusCode >= 300 {
			return nil, &APIError{
				StatusCode:  resp.StatusCode,
				Description: strings.TrimSpace(string(body)),
			}
		}
		return nil, fmt.Errorf("decode getUpdates response: %w", err)
	}

	if resp.StatusCode >= 300 || !payload.OK {
		statusCode := resp.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusBadGateway
		}
		return nil, &APIError{
			StatusCode:  statusCode,
			Description: strings.TrimSpace(payload.Description),
		}
	}

	return payload.Result, nil
}

func matchTargetChatCandidates(updates []botUpdate, telegramUserID int64) []TargetChatCandidate {
	if telegramUserID == 0 {
		return nil
	}

	byChatID := make(map[string]targetChatCandidateRecord)
	for _, update := range updates {
		if update.Message == nil || update.Message.From == nil {
			continue
		}
		if update.Message.From.ID != telegramUserID {
			continue
		}
		if update.Message.Chat.ID == 0 {
			continue
		}

		record := targetChatCandidateRecord{
			TargetChatCandidate: TargetChatCandidate{
				ChatID:   strconv.FormatInt(update.Message.Chat.ID, 10),
				ChatType: strings.TrimSpace(update.Message.Chat.Type),
				Title:    resolveChatTitle(update.Message.Chat),
				Username: strings.TrimSpace(update.Message.Chat.Username),
			},
			messageDate: update.Message.Date,
			updateID:    update.UpdateID,
		}

		existing, ok := byChatID[record.ChatID]
		if ok && !isNewerTargetChatCandidate(record, existing) {
			continue
		}
		byChatID[record.ChatID] = record
	}

	candidates := make([]targetChatCandidateRecord, 0, len(byChatID))
	for _, candidate := range byChatID {
		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].messageDate == candidates[j].messageDate {
			return candidates[i].updateID > candidates[j].updateID
		}
		return candidates[i].messageDate > candidates[j].messageDate
	})

	out := make([]TargetChatCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.TargetChatCandidate)
	}
	return out
}

func isNewerTargetChatCandidate(current, existing targetChatCandidateRecord) bool {
	if current.messageDate == existing.messageDate {
		return current.updateID > existing.updateID
	}
	return current.messageDate > existing.messageDate
}

func resolveChatTitle(chat botChat) string {
	if title := strings.TrimSpace(chat.Title); title != "" {
		return title
	}

	fullName := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(chat.FirstName),
		strings.TrimSpace(chat.LastName),
	}, " "))
	if fullName != "" {
		return fullName
	}

	switch strings.TrimSpace(chat.Type) {
	case "private":
		return "与 Bot 的私聊"
	case "group":
		return "群聊"
	case "supergroup":
		return "超级群"
	case "channel":
		return "频道"
	default:
		return "未命名会话"
	}
}
