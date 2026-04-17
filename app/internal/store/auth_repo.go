package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepository struct {
	pool   *pgxpool.Pool
	cipher Cipher
}

func (r *AuthRepository) Get(ctx context.Context) (*model.TelegramAuth, error) {
	var row model.TelegramAuth
	var encrypted string
	err := r.pool.QueryRow(ctx, `
		select id, phone_number, telegram_user_id, telegram_name, telegram_handle,
		       session_data, status, last_connected_at, created_at, updated_at
		from telegram_auth
		order by id desc
		limit 1
	`).Scan(
		&row.ID,
		&row.PhoneNumber,
		&row.TelegramUserID,
		&row.TelegramName,
		&row.TelegramHandle,
		&encrypted,
		&row.Status,
		&row.LastConnectedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query auth: %w", err)
	}

	row.SessionData, err = r.cipher.DecryptBytes(encrypted)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *AuthRepository) Save(ctx context.Context, auth model.TelegramAuth) error {
	encrypted, err := r.cipher.EncryptBytes(auth.SessionData)
	if err != nil {
		return err
	}

	current, err := r.Get(ctx)
	if err != nil {
		return err
	}

	if current == nil {
		_, err = r.pool.Exec(ctx, `
			insert into telegram_auth (
				phone_number, telegram_user_id, telegram_name, telegram_handle,
				session_data, status, last_connected_at
			) values ($1, $2, $3, $4, $5, $6, $7)
		`,
			auth.PhoneNumber,
			auth.TelegramUserID,
			auth.TelegramName,
			auth.TelegramHandle,
			encrypted,
			auth.Status,
			auth.LastConnectedAt,
		)
		if err != nil {
			return fmt.Errorf("insert auth: %w", err)
		}
		return nil
	}

	_, err = r.pool.Exec(ctx, `
		update telegram_auth
		set phone_number = $1,
		    telegram_user_id = $2,
		    telegram_name = $3,
		    telegram_handle = $4,
		    session_data = $5,
		    status = $6,
		    last_connected_at = $7,
		    updated_at = now()
		where id = $8
	`,
		auth.PhoneNumber,
		auth.TelegramUserID,
		auth.TelegramName,
		auth.TelegramHandle,
		encrypted,
		auth.Status,
		auth.LastConnectedAt,
		current.ID,
	)
	if err != nil {
		return fmt.Errorf("update auth: %w", err)
	}
	return nil
}

func (r *AuthRepository) Clear(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `delete from telegram_auth`)
	if err != nil {
		return fmt.Errorf("clear auth: %w", err)
	}
	return nil
}
