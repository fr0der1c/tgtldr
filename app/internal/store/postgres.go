package store

import (
	"context"
	"fmt"
	"time"

	"github.com/frederic/tgtldr/app/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool          *pgxpool.Pool
	Cipher        Cipher
	Settings      *SettingsRepository
	Auth          *AuthRepository
	LocalAuth     *LocalAuthRepository
	LocalSessions *LocalSessionRepository
	Chats         *ChatRepository
	Messages      *MessageRepository
	Summaries     *SummaryRepository
}

func Open(ctx context.Context, cfg config.Config) (*Store, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := waitForDatabase(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	cipher, err := NewCipher(cfg.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	st := &Store{
		Pool:          pool,
		Cipher:        cipher,
		Settings:      &SettingsRepository{pool: pool, cipher: cipher},
		Auth:          &AuthRepository{pool: pool, cipher: cipher},
		LocalAuth:     &LocalAuthRepository{pool: pool},
		LocalSessions: &LocalSessionRepository{pool: pool},
		Chats:         &ChatRepository{pool: pool},
		Messages:      &MessageRepository{pool: pool},
		Summaries:     &SummaryRepository{pool: pool},
	}
	return st, nil
}

func (s *Store) Close() {
	s.Pool.Close()
}

func waitForDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("ping postgres: %w", ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("ping postgres timeout: %w", err)
		case <-ticker.C:
		}
	}
}
