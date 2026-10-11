// Package alerts 在实时消息命中群组关键词时通过 Telegram Bot 推送提醒。
package alerts

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

// Cooldown 是同一群组同一关键词两次提醒之间的最短间隔，避免刷屏。
const Cooldown = 10 * time.Minute

// maxMessageAge 之外的消息不再提醒，避免重连后补收的停机期间消息集中推送。
const maxMessageAge = 10 * time.Minute

const excerptLimit = 300

type sender interface {
	SendMessageWithLanguage(ctx context.Context, token, chatID, text string, language model.Language) error
}

type cooldownKey struct {
	chatID  int64
	keyword string
}

type Service struct {
	store *store.Store
	clock clock.Clock
	bot   sender

	mu        sync.Mutex
	lastAlert map[cooldownKey]time.Time
}

func NewService(st *store.Store, c clock.Clock, bot sender) *Service {
	return &Service{
		store:     st,
		clock:     c,
		bot:       bot,
		lastAlert: make(map[cooldownKey]time.Time),
	}
}

// Handle 检查一条刚保存的实时消息，命中关键词且未处于冷却期时推送提醒；失败只影响本次提醒。
func (s *Service) Handle(ctx context.Context, chat model.Chat, message model.Message) error {
	if !chat.AlertEnabled || s.clock.Now().Sub(message.MessageTime) > maxMessageAge {
		return nil
	}
	keyword, ok := MatchKeyword(chat.AlertKeywords, message.TextContent+"\n"+message.Caption)
	if !ok {
		return nil
	}
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if !botReady(settings) {
		return nil
	}
	claimedAt, ok := s.claim(chat.ID, keyword)
	if !ok {
		return nil
	}

	text := BuildMessage(chat, message, keyword, settings.Language)
	if err := s.bot.SendMessageWithLanguage(ctx, settings.BotToken, settings.BotTargetChatID, text, settings.Language); err != nil {
		// 未送达的提醒不应占用冷却期，否则后续命中会被一并吞掉。
		s.release(chat.ID, keyword, claimedAt)
		log.Printf("keyword alert delivery failed: chat_id=%d error=%v", chat.ID, err)
		return err
	}
	return nil
}

func botReady(settings model.AppSettings) bool {
	return settings.BotEnabled &&
		strings.TrimSpace(settings.BotToken) != "" &&
		strings.TrimSpace(settings.BotTargetChatID) != ""
}

// claim 在冷却期外登记本次提醒并返回登记时间；返回 false 表示应跳过。
func (s *Service) claim(chatID int64, keyword string) (time.Time, bool) {
	key := newCooldownKey(chatID, keyword)
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if last, ok := s.lastAlert[key]; ok && now.Sub(last) < Cooldown {
		return time.Time{}, false
	}
	s.lastAlert[key] = now
	return now, true
}

// release 撤销一次未送达的登记；只在记录仍是本次登记时删除，避免覆盖并发的成功提醒。
func (s *Service) release(chatID int64, keyword string, claimedAt time.Time) {
	key := newCooldownKey(chatID, keyword)
	s.mu.Lock()
	defer s.mu.Unlock()
	if last, ok := s.lastAlert[key]; ok && last.Equal(claimedAt) {
		delete(s.lastAlert, key)
	}
}

func newCooldownKey(chatID int64, keyword string) cooldownKey {
	return cooldownKey{chatID: chatID, keyword: strings.ToLower(keyword)}
}

// MatchKeyword 不区分大小写地返回第一个出现在正文中的关键词。
func MatchKeyword(keywords []string, text string) (string, bool) {
	lowered := strings.ToLower(text)
	for _, keyword := range keywords {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		if strings.Contains(lowered, strings.ToLower(trimmed)) {
			return trimmed, true
		}
	}
	return "", false
}

// BuildMessage 生成提醒正文，正文摘录按字符截断以免超出 Telegram 限制。
func BuildMessage(chat model.Chat, message model.Message, keyword string, language model.Language) string {
	content := strings.TrimSpace(message.TextContent)
	if content == "" {
		content = strings.TrimSpace(message.Caption)
	}
	content = truncateRunes(content, excerptLimit)
	sender := strings.TrimSpace(message.SenderName)
	if language == model.LanguageEN {
		return fmt.Sprintf("**Keyword alert · %s**\n\nKeyword: %s\nSender: %s\n\n%s", chat.Title, keyword, sender, content)
	}
	return fmt.Sprintf("**关键词提醒 · %s**\n\n关键词：%s\n发送者：%s\n\n%s", chat.Title, keyword, sender, content)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
