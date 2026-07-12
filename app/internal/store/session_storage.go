package store

import (
	"context"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/gotd/td/session"
)

type SessionStorage struct {
	auth      *AuthRepository
	accountID int64
}

func NewSessionStorage(auth *AuthRepository, accountID int64) *SessionStorage {
	return &SessionStorage{auth: auth, accountID: accountID}
}

func (s *SessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	current, err := s.auth.GetByID(ctx, s.accountID)
	if err != nil {
		return nil, err
	}
	if current == nil || len(current.SessionData) == 0 {
		return nil, session.ErrNotFound
	}
	return current.SessionData, nil
}

func (s *SessionStorage) StoreSession(ctx context.Context, data []byte) error {
	current, err := s.auth.GetByID(ctx, s.accountID)
	if err != nil {
		return err
	}
	if current == nil {
		current = &model.TelegramAuth{ID: s.accountID, Status: "logged_in"}
	}
	current.SessionData = data
	return s.auth.Save(ctx, *current)
}
