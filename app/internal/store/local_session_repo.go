package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LocalSessionRepository struct {
	pool *pgxpool.Pool
}

func (r *LocalSessionRepository) Create(ctx context.Context, session model.LocalSession) error {
	_, err := r.pool.Exec(ctx, `
		insert into local_sessions (session_id, session_version, expires_at, last_seen_at)
		values ($1, $2, $3, $4)
	`, session.SessionID, session.SessionVersion, session.ExpiresAt, session.LastSeenAt)
	if err != nil {
		return fmt.Errorf("insert local session: %w", err)
	}
	return nil
}

func (r *LocalSessionRepository) GetBySessionID(ctx context.Context, sessionID string) (*model.LocalSession, error) {
	var session model.LocalSession
	err := r.pool.QueryRow(ctx, `
		select id, session_id, session_version, expires_at, last_seen_at, created_at, updated_at
		from local_sessions
		where session_id = $1
	`, sessionID).Scan(
		&session.ID,
		&session.SessionID,
		&session.SessionVersion,
		&session.ExpiresAt,
		&session.LastSeenAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query local session: %w", err)
	}
	return &session, nil
}

func (r *LocalSessionRepository) Touch(ctx context.Context, sessionID string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		update local_sessions
		set last_seen_at = $1,
		    updated_at = now()
		where session_id = $2
	`, now, sessionID)
	if err != nil {
		return fmt.Errorf("touch local session: %w", err)
	}
	return nil
}

func (r *LocalSessionRepository) Delete(ctx context.Context, sessionID string) error {
	_, err := r.pool.Exec(ctx, `delete from local_sessions where session_id = $1`, sessionID)
	if err != nil {
		return fmt.Errorf("delete local session: %w", err)
	}
	return nil
}

func (r *LocalSessionRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := r.pool.Exec(ctx, `delete from local_sessions where expires_at <= $1`, now)
	if err != nil {
		return fmt.Errorf("delete expired local sessions: %w", err)
	}
	return nil
}

func (r *LocalSessionRepository) DeleteByVersionMismatch(ctx context.Context, version int) error {
	_, err := r.pool.Exec(ctx, `delete from local_sessions where session_version <> $1`, version)
	if err != nil {
		return fmt.Errorf("delete stale local sessions: %w", err)
	}
	return nil
}
