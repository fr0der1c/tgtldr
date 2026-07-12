package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

var (
	ErrConfigIncomplete = errors.New("telegram api 配置不完整")
	ErrAuthNotStarted   = errors.New("认证尚未开始")
	ErrPasswordNeeded   = errors.New("需要 2FA 密码")

	errTelegramUnauthorized = errors.New("telegram session not authorized")
)

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

	historyBackfills *historyBackfillStore

	historyBackfillCompleted func(chat model.Chat, fromDate, toDate string)

	mu        sync.Mutex
	pending   *model.AuthSessionState
	listeners map[int64]context.CancelFunc
}

func NewService(root context.Context, st *store.Store, c clock.Clock) *Service {
	return &Service{
		store:            st,
		clock:            c,
		root:             root,
		historyBackfills: newHistoryBackfillStore(),
		listeners:        make(map[int64]context.CancelFunc),
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

func (s *Service) SyncChats(ctx context.Context, accountIDs ...int64) error {
	accountID, err := s.resolveAccountID(ctx, accountIDs...)
	if err != nil {
		return err
	}
	client, _, err := s.newClient(accountID)
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
			return s.markAuthLoggedOut(ctx, accountID)
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
		if authErr := s.markAuthLoggedOutOnInvalidSession(ctx, accountID, err); authErr != err {
			return fmt.Errorf("sync chats from telegram: %w", authErr)
		}
		return fmt.Errorf("sync chats from telegram: %w", err)
	}

	for _, chat := range chats {
		stored, err := s.store.Chats.EnsureExists(ctx, chat)
		if err != nil {
			return err
		}
		if err := s.store.AccountChats.Upsert(ctx, accountID, stored.ID, chat.TelegramAccess); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) EnsureListener(accountIDs ...int64) {
	accountID, err := s.resolveAccountID(context.Background(), accountIDs...)
	if err != nil {
		return
	}
	s.mu.Lock()
	if _, running := s.listeners[accountID]; running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.root)
	s.listeners[accountID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.listeners, accountID)
			s.mu.Unlock()
		}()
		s.runListenerLoop(ctx, accountID)
	}()
}

func (s *Service) StopListener() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.listeners {
		cancel()
	}
}

func (s *Service) StopAccount(accountID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel := s.listeners[accountID]; cancel != nil {
		cancel()
	}
}

func (s *Service) CancelAuth(accountID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil && s.pending.AccountID == accountID {
		s.pending = nil
	}
}

func (s *Service) runListenerLoop(ctx context.Context, accountID int64) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.runListener(ctx, accountID); err != nil {
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

func (s *Service) runListener(ctx context.Context, accountID int64) error {
	client, _, err := s.newClient(accountID)
	if err != nil {
		return err
	}

	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnNewMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateNewMessage) error {
		return s.onNewMessage(ctx, accountID, entities, update)
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateNewChannelMessage) error {
		return s.onNewChannelMessage(ctx, accountID, entities, update)
	})

	manager := updates.New(updates.Config{Handler: dispatcher})
	client, err = s.newConfiguredClient(accountID, manager)
	if err != nil {
		return err
	}

	err = client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized || status.User == nil {
			return s.markAuthLoggedOut(ctx, accountID)
		}
		if err := manager.Run(ctx, client.API(), status.User.ID, updates.AuthOptions{IsBot: false}); err != nil {
			if authorized := s.checkCurrentAuthStatus(ctx, client); !authorized {
				return s.markAuthLoggedOut(ctx, accountID)
			}
			return err
		}
		return nil
	})
	if err != nil {
		return s.markAuthLoggedOutOnInvalidSession(ctx, accountID, err)
	}
	return nil
}

func (s *Service) checkCurrentAuthStatus(ctx context.Context, client *telegram.Client) bool {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		log.Printf("telegram auth status check failed: %v", err)
		if isInvalidTelegramSessionError(err) {
			return false
		}
		return true
	}
	return status.Authorized && status.User != nil
}

func (s *Service) markAuthLoggedOut(ctx context.Context, accountID int64) error {
	current, err := s.store.Auth.GetByID(ctx, accountID)
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

func (s *Service) markAuthLoggedOutOnInvalidSession(ctx context.Context, accountID int64, err error) error {
	if !isInvalidTelegramSessionError(err) {
		return err
	}
	if logoutErr := s.markAuthLoggedOut(ctx, accountID); logoutErr != nil {
		return logoutErr
	}
	return errTelegramUnauthorized
}

func isInvalidTelegramSessionError(err error) bool {
	if err == nil {
		return false
	}
	if auth.IsUnauthorized(err) {
		return true
	}
	return tgerr.Is(err, "AUTH_KEY_UNREGISTERED", "SESSION_EXPIRED", "AUTH_KEY_DUPLICATED")
}

func (s *Service) onNewMessage(ctx context.Context, accountID int64, entities tg.Entities, update *tg.UpdateNewMessage) error {
	return s.storeIncomingMessage(ctx, accountID, entities, update.Message)
}

func (s *Service) onNewChannelMessage(ctx context.Context, accountID int64, entities tg.Entities, update *tg.UpdateNewChannelMessage) error {
	return s.storeIncomingMessage(ctx, accountID, entities, update.Message)
}

func (s *Service) storeIncomingMessage(ctx context.Context, accountID int64, entities tg.Entities, messageClass tg.MessageClass) error {
	msg, ok := messageClass.(*tg.Message)
	if !ok || msg.Out {
		return nil
	}

	telegramChatID, chatType, ok := extractChat(msg.PeerID)
	if !ok || (chatType != "group" && chatType != "supergroup") {
		return nil
	}

	chat, err := s.store.Chats.GetByTelegramID(ctx, telegramChatID)
	if err != nil {
		if store.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !chat.Enabled || chat.CollectorAccountID != accountID {
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
	return s.store.Messages.Upsert(ctx, item)
}

func (s *Service) persistAuthorizedUser(ctx context.Context, client *telegram.Client, accountID int64, phone string) error {
	self, err := client.Self(ctx)
	if err != nil {
		return fmt.Errorf("fetch self after login: %w", err)
	}

	current, err := s.store.Auth.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if current == nil {
		current = &model.TelegramAuth{ID: accountID}
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

func (s *Service) resolveAccountID(ctx context.Context, accountIDs ...int64) (int64, error) {
	if len(accountIDs) > 0 && accountIDs[0] != 0 {
		return accountIDs[0], nil
	}
	current, err := s.store.Auth.Get(ctx)
	if err != nil {
		return 0, err
	}
	if current == nil {
		return 0, errTelegramUnauthorized
	}
	return current.ID, nil
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
