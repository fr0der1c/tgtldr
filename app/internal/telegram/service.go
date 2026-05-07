package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/frederic/tgtldr/app/internal/bot"
	"github.com/frederic/tgtldr/app/internal/clock"
	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/frederic/tgtldr/app/internal/store"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
)

var (
	ErrConfigIncomplete = errors.New("telegram api 配置不完整")
	ErrAuthNotStarted   = errors.New("认证尚未开始")
	ErrPasswordNeeded   = errors.New("需要 2FA 密码")

	errTelegramUnauthorized = errors.New("telegram session not authorized")
)

const keywordAlertCooldown = 10 * time.Minute

type FloodWaitError struct {
	Wait time.Duration
}

func (e *FloodWaitError) Error() string {
	seconds := e.RetryAfterSeconds()
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("Telegram 暂时限制了请求，请在 %d 秒后重试。", seconds)
}

func (e *FloodWaitError) RetryAfterSeconds() int {
	seconds := int(e.Wait.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return seconds
}

type Service struct {
	store *store.Store
	clock clock.Clock
	root  context.Context
	bot   *bot.Service

	historyBackfills *historyBackfillStore

	historyBackfillCompleted func(chat model.Chat, fromDate, toDate string)

	mu             sync.Mutex
	alertMu        sync.Mutex
	alertLastSent  map[string]time.Time
	pending        *model.AuthSessionState
	listenerCancel context.CancelFunc
	listenerRun    bool
}

func NewService(root context.Context, st *store.Store, c clock.Clock) *Service {
	return &Service{
		store:            st,
		clock:            c,
		root:             root,
		bot:              bot.New(),
		historyBackfills: newHistoryBackfillStore(),
		alertLastSent:    map[string]time.Time{},
	}
}

func (s *Service) PendingAuthState() *model.AuthSessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pending == nil {
		return nil
	}
	state := *s.pending
	return &state
}

func (s *Service) SetHistoryBackfillCompletionHook(fn func(chat model.Chat, fromDate, toDate string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.historyBackfillCompleted = fn
}

func (s *Service) historyBackfillCompletionHook() func(chat model.Chat, fromDate, toDate string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.historyBackfillCompleted
}

func (s *Service) StartAuth(ctx context.Context, phone string) (*model.AuthSessionState, error) {
	client, _, err := s.newClient()
	if err != nil {
		return nil, err
	}

	var next *model.AuthSessionState
	err = client.Run(ctx, func(ctx context.Context) error {
		sent, err := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		if err != nil {
			return wrapTelegramError(err)
		}

		code, ok := sent.(*tg.AuthSentCode)
		if !ok {
			return fmt.Errorf("unexpected sent code type %T", sent)
		}

		next = &model.AuthSessionState{
			Step:        model.AuthStepCode,
			PhoneNumber: phone,
			CodeHash:    code.PhoneCodeHash,
			Deadline:    s.clock.Now().Add(10 * time.Minute),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("send telegram code: %w", err)
	}

	s.mu.Lock()
	s.pending = next
	s.mu.Unlock()
	return next, nil
}

func (s *Service) VerifyCode(ctx context.Context, code string) (*model.AuthSessionState, error) {
	client, _, err := s.newClient()
	if err != nil {
		return nil, err
	}

	pending, err := s.requirePending(model.AuthStepCode)
	if err != nil {
		return nil, err
	}

	var next *model.AuthSessionState
	err = client.Run(ctx, func(ctx context.Context) error {
		_, err := client.Auth().SignIn(ctx, pending.PhoneNumber, code, pending.CodeHash)
		switch {
		case errors.Is(err, auth.ErrPasswordAuthNeeded):
			next = &model.AuthSessionState{
				Step:        model.AuthStepPassword,
				PhoneNumber: pending.PhoneNumber,
				CodeHash:    pending.CodeHash,
				Deadline:    s.clock.Now().Add(10 * time.Minute),
			}
			return nil
		case err != nil:
			return wrapTelegramError(err)
		default:
			return s.persistAuthorizedUser(ctx, client, pending.PhoneNumber)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("sign in telegram: %w", err)
	}

	if next != nil {
		s.mu.Lock()
		s.pending = next
		s.mu.Unlock()
		return next, ErrPasswordNeeded
	}

	s.clearPending()
	if err := s.SyncChats(ctx); err != nil {
		return nil, err
	}
	s.EnsureListener()
	return &model.AuthSessionState{Step: model.AuthStepDone, PhoneNumber: pending.PhoneNumber}, nil
}

func (s *Service) VerifyPassword(ctx context.Context, password string) (*model.AuthSessionState, error) {
	client, _, err := s.newClient()
	if err != nil {
		return nil, err
	}

	pending, err := s.requirePending(model.AuthStepPassword)
	if err != nil {
		return nil, err
	}

	err = client.Run(ctx, func(ctx context.Context) error {
		if _, err := client.Auth().Password(ctx, strings.TrimSpace(password)); err != nil {
			return wrapTelegramError(err)
		}
		return s.persistAuthorizedUser(ctx, client, pending.PhoneNumber)
	})
	if err != nil {
		return nil, fmt.Errorf("submit telegram password: %w", err)
	}

	s.clearPending()
	if err := s.SyncChats(ctx); err != nil {
		return nil, err
	}
	s.EnsureListener()
	return &model.AuthSessionState{Step: model.AuthStepDone, PhoneNumber: pending.PhoneNumber}, nil
}

func (s *Service) SyncChats(ctx context.Context) error {
	client, _, err := s.newClient()
	if err != nil {
		return err
	}

	var chats []model.Chat
	err = client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return s.markAuthLoggedOut(ctx)
		}

		builder := dialogsquery.NewQueryBuilder(client.API()).GetDialogs().BatchSize(100)
		if err := builder.ForEach(ctx, func(_ context.Context, elem dialogsquery.Elem) error {
			chat, ok := dialogToChat(elem)
			if !ok {
				return nil
			}
			chats = append(chats, chat)
			return nil
		}); err != nil {
			return wrapTelegramError(err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("sync chats from telegram: %w", err)
	}

	return s.store.Chats.UpsertMany(ctx, chats)
}

func (s *Service) EnsureListener() {
	s.mu.Lock()
	if s.listenerRun {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.root)
	s.listenerCancel = cancel
	s.listenerRun = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.listenerRun = false
			s.listenerCancel = nil
			s.mu.Unlock()
		}()
		s.runListenerLoop(ctx)
	}()
}

func (s *Service) StopListener() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listenerCancel != nil {
		s.listenerCancel()
	}
}

func (s *Service) runListenerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.runListener(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, errTelegramUnauthorized) {
				log.Printf("telegram listener stopped: %v", err)
				return
			}
			log.Printf("telegram listener error: %v; retrying in 5s", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}
		return
	}
}

func (s *Service) runListener(ctx context.Context) error {
	client, _, err := s.newClient()
	if err != nil {
		return err
	}

	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnNewMessage(s.onNewMessage)
	dispatcher.OnNewChannelMessage(s.onNewChannelMessage)

	manager := updates.New(updates.Config{Handler: dispatcher})
	client = s.newConfiguredClient(manager)

	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized || status.User == nil {
			return s.markAuthLoggedOut(ctx)
		}
		if err := manager.Run(ctx, client.API(), status.User.ID, updates.AuthOptions{IsBot: false}); err != nil {
			if authorized := s.checkCurrentAuthStatus(ctx, client); !authorized {
				return s.markAuthLoggedOut(ctx)
			}
			return err
		}
		return nil
	})
}

func (s *Service) checkCurrentAuthStatus(ctx context.Context, client *telegram.Client) bool {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		log.Printf("telegram auth status check failed: %v", err)
		return true
	}
	return status.Authorized && status.User != nil
}

func (s *Service) markAuthLoggedOut(ctx context.Context) error {
	current, err := s.store.Auth.Get(ctx)
	if err != nil {
		return fmt.Errorf("load telegram auth before logout: %w", err)
	}
	if current == nil {
		return errTelegramUnauthorized
	}
	next := loggedOutAuth(*current)
	if err := s.store.Auth.Save(ctx, next); err != nil {
		return fmt.Errorf("mark telegram auth logged out: %w", err)
	}
	return errTelegramUnauthorized
}

func loggedOutAuth(current model.TelegramAuth) model.TelegramAuth {
	current.Status = "logged_out"
	current.SessionData = nil
	return current
}

func (s *Service) onNewMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateNewMessage) error {
	return s.storeIncomingMessage(ctx, entities, update.Message)
}

func (s *Service) onNewChannelMessage(ctx context.Context, entities tg.Entities, update *tg.UpdateNewChannelMessage) error {
	return s.storeIncomingMessage(ctx, entities, update.Message)
}

func (s *Service) storeIncomingMessage(ctx context.Context, entities tg.Entities, messageClass tg.MessageClass) error {
	msg, ok := messageClass.(*tg.Message)
	if !ok || msg.Out {
		return nil
	}

	telegramChatID, _, ok := extractChat(msg.PeerID)
	if !ok {
		return nil
	}

	chat, err := s.store.Chats.GetByTelegramID(ctx, telegramChatID)
	if err != nil {
		if store.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !chat.Enabled {
		return nil
	}

	payload, _ := json.Marshal(msg)
	senderID, senderName, senderUsername, senderIsBot := resolveSender(msg, entities)
	item := model.Message{
		ChatID:            chat.ID,
		TelegramMessageID: msg.ID,
		TelegramSenderID:  senderID,
		SenderName:        senderName,
		SenderUsername:    senderUsername,
		SenderIsBot:       senderIsBot,
		TextContent:       msg.Message,
		Caption:           extractCaption(msg),
		MessageType:       classifyMessage(msg),
		MediaKind:         mediaKind(msg),
		ReplyToMessageID:  replyToID(msg),
		MessageTime:       time.Unix(int64(msg.Date), 0).UTC(),
		RawJSON:           string(payload),
	}
	if err := s.store.Messages.Upsert(ctx, item); err != nil {
		return err
	}
	s.maybeSendKeywordAlert(ctx, chat, item)
	return nil
}

func (s *Service) persistAuthorizedUser(ctx context.Context, client *telegram.Client, phone string) error {
	self, err := client.Self(ctx)
	if err != nil {
		return fmt.Errorf("fetch self after login: %w", err)
	}

	current, err := s.store.Auth.Get(ctx)
	if err != nil {
		return err
	}
	if current == nil {
		current = &model.TelegramAuth{}
	}

	current.PhoneNumber = phone
	current.TelegramUserID = self.ID
	current.TelegramName = strings.TrimSpace(strings.TrimSpace(self.FirstName + " " + self.LastName))
	current.TelegramHandle = self.Username
	current.Status = "authorized"
	current.LastConnectedAt = s.clock.Now()
	return s.store.Auth.Save(ctx, *current)
}

func (s *Service) BootstrapAuth(ctx context.Context) (*model.TelegramAuth, error) {
	return s.store.Auth.Get(ctx)
}

func (s *Service) newClient() (*telegram.Client, model.AppSettings, error) {
	settings, err := s.store.Settings.Get(context.Background())
	if err != nil {
		return nil, model.AppSettings{}, err
	}
	if settings.TelegramAPIID == 0 || strings.TrimSpace(settings.TelegramAPIHash) == "" {
		return nil, model.AppSettings{}, ErrConfigIncomplete
	}
	client := s.newConfiguredClient(nil)
	return client, settings, nil
}

func (s *Service) newConfiguredClient(handler telegram.UpdateHandler) *telegram.Client {
	settings, _ := s.store.Settings.Get(context.Background())
	options := telegram.Options{
		SessionStorage: store.NewSessionStorage(s.store.Auth),
		UpdateHandler:  handler,
		Device: telegram.DeviceConfig{
			DeviceModel:    "TGTLDR",
			SystemVersion:  "Desktop",
			AppVersion:     "Self-hosted",
			SystemLangCode: "zh",
			LangCode:       "zh",
		},
	}
	return telegram.NewClient(settings.TelegramAPIID, settings.TelegramAPIHash, options)
}

func wrapTelegramError(err error) error {
	if wait, ok := telegram.AsFloodWait(err); ok {
		return &FloodWaitError{Wait: wait}
	}
	return err
}

func (s *Service) clearPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = nil
}

func (s *Service) requirePending(step model.AuthStep) (*model.AuthSessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pending == nil || s.pending.Step != step {
		return nil, ErrAuthNotStarted
	}
	if s.clock.Now().After(s.pending.Deadline) {
		s.pending = nil
		return nil, fmt.Errorf("认证会话已过期")
	}
	state := *s.pending
	return &state, nil
}

func (s *Service) maybeSendKeywordAlert(ctx context.Context, chat model.Chat, item model.Message) {
	if !chat.AlertEnabled || len(chat.AlertKeywords) == 0 {
		return
	}
	if item.SenderIsBot {
		return
	}

	text := strings.TrimSpace(item.SummaryText())
	if text == "" {
		return
	}

	matched, ok := matchAlertKeyword(text, chat.AlertKeywords)
	if !ok {
		return
	}
	if !s.allowKeywordAlert(chat.ID, matched) {
		return
	}

	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		log.Printf("keyword alert skipped: load settings failed: %v", err)
		return
	}
	if !settings.BotEnabled || strings.TrimSpace(settings.BotToken) == "" || strings.TrimSpace(settings.BotTargetChatID) == "" {
		return
	}

	message := formatKeywordAlertMessage(chat, item, matched)
	if err := s.bot.SendMessageWithLanguage(
		ctx,
		settings.BotToken,
		settings.BotTargetChatID,
		message,
		model.NormalizeLanguage(settings.Language),
	); err != nil {
		log.Printf("keyword alert send failed: chat=%d keyword=%q err=%v", chat.ID, matched, err)
		return
	}
}

func matchAlertKeyword(text string, keywords []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, keyword := range keywords {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(trimmed)) {
			return trimmed, true
		}
	}
	return "", false
}

func (s *Service) allowKeywordAlert(chatID int64, keyword string) bool {
	key := strconv.FormatInt(chatID, 10) + "|" + strings.ToLower(strings.TrimSpace(keyword))
	now := s.clock.Now()

	s.alertMu.Lock()
	defer s.alertMu.Unlock()

	last, ok := s.alertLastSent[key]
	if ok && now.Sub(last) < keywordAlertCooldown {
		return false
	}
	s.alertLastSent[key] = now
	return true
}

func formatKeywordAlertMessage(chat model.Chat, item model.Message, keyword string) string {
	content := strings.TrimSpace(item.SummaryText())
	if len(content) > 300 {
		content = content[:300] + "..."
	}

	lines := []string{
		"关键词提醒",
		"会话：" + strings.TrimSpace(chat.Title),
		"关键词：" + strings.TrimSpace(keyword),
		"发送者：" + strings.TrimSpace(item.SenderName),
		"时间(UTC)：" + item.MessageTime.UTC().Format("2006-01-02 15:04:05"),
		"内容：" + content,
	}

	if link := telegramMessageLink(chat, item.TelegramMessageID); link != "" {
		lines = append(lines, "链接："+link)
	}
	return strings.Join(lines, "\n")
}

func telegramMessageLink(chat model.Chat, messageID int) string {
	username := strings.TrimSpace(chat.Username)
	if username == "" || messageID <= 0 {
		return ""
	}
	return "https://t.me/" + username + "/" + strconv.Itoa(messageID)
}
