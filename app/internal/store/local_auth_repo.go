package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LocalAuthRepository struct {
	pool *pgxpool.Pool
}

func (r *LocalAuthRepository) Get(ctx context.Context) (*model.LocalAuth, error) {
	var auth model.LocalAuth
	err := r.pool.QueryRow(ctx, `
		select password_hash, password_updated_at, session_version, created_at, updated_at
		from local_auth
		order by id
		limit 1
	`).Scan(
		&auth.PasswordHash,
		&auth.PasswordUpdatedAt,
		&auth.SessionVersion,
		&auth.CreatedAt,
		&auth.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query local auth: %w", err)
	}
	return &auth, nil
}

func (r *LocalAuthRepository) Save(ctx context.Context, auth model.LocalAuth) error {
	current, err := r.Get(ctx)
	if err != nil {
		return err
	}

	if current == nil {
		_, err = r.pool.Exec(ctx, `
			insert into local_auth (password_hash, password_updated_at, session_version)
			values ($1, $2, $3)
		`, auth.PasswordHash, auth.PasswordUpdatedAt, auth.SessionVersion)
		if err != nil {
			return fmt.Errorf("insert local auth: %w", err)
		}
		return nil
	}

	_, err = r.pool.Exec(ctx, `
		update local_auth
		set password_hash = $1,
		    password_updated_at = $2,
		    session_version = $3,
		    updated_at = now()
		where id = (select id from local_auth order by id limit 1)
	`, auth.PasswordHash, auth.PasswordUpdatedAt, auth.SessionVersion)
	if err != nil {
		return fmt.Errorf("update local auth: %w", err)
	}
	return nil
}
