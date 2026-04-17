package store

import (
	"context"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/gotd/td/session"
)

type SessionStorage struct {
	auth *AuthRepository
}

func NewSessionStorage(auth *AuthRepository) *SessionStorage {
	return &SessionStorage{auth: auth}
}

func (s *SessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	current, err := s.auth.Get(ctx)
	if err != nil {
		return nil, err
	}
	if current == nil || len(current.SessionData) == 0 {
		return nil, session.ErrNotFound
	}
	return current.SessionData, nil
}

func (s *SessionStorage) StoreSession(ctx context.Context, data []byte) error {
	current, err := s.auth.Get(ctx)
	if err != nil {
		return err
	}
	if current == nil {
		current = &model.TelegramAuth{Status: "logged_in"}
	}
	current.SessionData = data
	return s.auth.Save(ctx, *current)
}
