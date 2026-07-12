package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

func (s *Service) StartAuth(ctx context.Context, phone string, accountIDs ...int64) (*model.AuthSessionState, error) {
	account, created, err := s.prepareAuthAccount(ctx, phone, accountIDs...)
	if err != nil {
		return nil, err
	}
	client, _, err := s.newClient(account.ID)
	if err != nil {
		s.cleanupFailedAuth(ctx, account, created)
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
			AccountID: account.ID, Step: model.AuthStepCode, PhoneNumber: phone,
			CodeHash: code.PhoneCodeHash, Deadline: s.clock.Now().Add(10 * time.Minute),
		}
		return nil
	})
	if err != nil {
		s.cleanupFailedAuth(ctx, account, created)
		return nil, fmt.Errorf("send telegram code: %w", err)
	}
	s.mu.Lock()
	s.pending = next
	s.mu.Unlock()
	return next, nil
}

func (s *Service) prepareAuthAccount(ctx context.Context, phone string, accountIDs ...int64) (model.TelegramAuth, bool, error) {
	if len(accountIDs) == 0 || accountIDs[0] == 0 {
		account, err := s.store.Auth.Create(ctx, phone)
		return account, true, err
	}
	current, err := s.store.Auth.GetByID(ctx, accountIDs[0])
	if err != nil {
		return model.TelegramAuth{}, false, err
	}
	if current == nil {
		return model.TelegramAuth{}, false, fmt.Errorf("telegram account %d not found", accountIDs[0])
	}
	current.PhoneNumber = phone
	current.SessionData = nil
	current.Status = "authorizing"
	if err := s.store.Auth.Save(ctx, *current); err != nil {
		return model.TelegramAuth{}, false, err
	}
	return *current, false, nil
}

func (s *Service) cleanupFailedAuth(ctx context.Context, account model.TelegramAuth, created bool) {
	if created {
		_ = s.store.Auth.Delete(ctx, account.ID)
		return
	}
	account.Status = "logged_out"
	account.SessionData = nil
	_ = s.store.Auth.Save(ctx, account)
}

func (s *Service) CancelPendingAuth(ctx context.Context) error {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()
	if pending == nil {
		return nil
	}
	account, err := s.store.Auth.GetByID(ctx, pending.AccountID)
	if err != nil || account == nil {
		return err
	}
	if account.TelegramUserID == 0 {
		return s.store.Auth.Delete(ctx, account.ID)
	}
	account.Status = "logged_out"
	account.SessionData = nil
	return s.store.Auth.Save(ctx, *account)
}

func (s *Service) VerifyCode(ctx context.Context, code string) (*model.AuthSessionState, error) {
	pending, err := s.requirePending(model.AuthStepCode)
	if err != nil {
		return nil, err
	}
	client, _, err := s.newClient(pending.AccountID)
	if err != nil {
		return nil, err
	}
	var next *model.AuthSessionState
	err = client.Run(ctx, func(ctx context.Context) error {
		_, err := client.Auth().SignIn(ctx, pending.PhoneNumber, code, pending.CodeHash)
		switch {
		case errors.Is(err, auth.ErrPasswordAuthNeeded):
			next = &model.AuthSessionState{
				AccountID: pending.AccountID, Step: model.AuthStepPassword,
				PhoneNumber: pending.PhoneNumber, CodeHash: pending.CodeHash,
				Deadline: s.clock.Now().Add(10 * time.Minute),
			}
			return nil
		case err != nil:
			return wrapTelegramError(err)
		default:
			return s.persistAuthorizedUser(ctx, client, pending.AccountID, pending.PhoneNumber)
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
	return s.finishAuth(ctx, pending)
}

func (s *Service) VerifyPassword(ctx context.Context, password string) (*model.AuthSessionState, error) {
	pending, err := s.requirePending(model.AuthStepPassword)
	if err != nil {
		return nil, err
	}
	client, _, err := s.newClient(pending.AccountID)
	if err != nil {
		return nil, err
	}
	err = client.Run(ctx, func(ctx context.Context) error {
		if _, err := client.Auth().Password(ctx, strings.TrimSpace(password)); err != nil {
			return wrapTelegramError(err)
		}
		return s.persistAuthorizedUser(ctx, client, pending.AccountID, pending.PhoneNumber)
	})
	if err != nil {
		return nil, fmt.Errorf("submit telegram password: %w", err)
	}
	return s.finishAuth(ctx, pending)
}

func (s *Service) finishAuth(ctx context.Context, pending *model.AuthSessionState) (*model.AuthSessionState, error) {
	s.clearPending()
	if err := s.SyncChats(ctx, pending.AccountID); err != nil {
		return nil, err
	}
	s.EnsureListener(pending.AccountID)
	return &model.AuthSessionState{
		AccountID: pending.AccountID, Step: model.AuthStepDone, PhoneNumber: pending.PhoneNumber,
	}, nil
}
