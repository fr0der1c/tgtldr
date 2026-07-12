package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/fr0der1c/tgtldr/app/internal/model"
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
		from telegram_accounts
		where status = 'authorized'
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

func (r *AuthRepository) List(ctx context.Context) ([]model.TelegramAuth, error) {
	rows, err := r.pool.Query(ctx, `
		select a.id, a.phone_number, a.telegram_user_id, a.telegram_name, a.telegram_handle,
		       a.session_data, a.status,
		       (select count(*) from chats c where c.collector_account_id = a.id),
		       a.last_connected_at, a.created_at, a.updated_at
		from telegram_accounts a
		order by a.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list telegram accounts: %w", err)
	}
	defer rows.Close()
	out := make([]model.TelegramAuth, 0)
	for rows.Next() {
		var item model.TelegramAuth
		var encrypted string
		if err := rows.Scan(&item.ID, &item.PhoneNumber, &item.TelegramUserID, &item.TelegramName,
			&item.TelegramHandle, &encrypted, &item.Status, &item.UsedByChatCount,
			&item.LastConnectedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan telegram account: %w", err)
		}
		item.SessionData, err = r.cipher.DecryptBytes(encrypted)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AuthRepository) GetByID(ctx context.Context, id int64) (*model.TelegramAuth, error) {
	var row model.TelegramAuth
	var encrypted string
	err := r.pool.QueryRow(ctx, `
		select id, phone_number, telegram_user_id, telegram_name, telegram_handle,
		       session_data, status, last_connected_at, created_at, updated_at
		from telegram_accounts where id = $1
	`, id).Scan(&row.ID, &row.PhoneNumber, &row.TelegramUserID, &row.TelegramName,
		&row.TelegramHandle, &encrypted, &row.Status, &row.LastConnectedAt, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get telegram account %d: %w", id, err)
	}
	row.SessionData, err = r.cipher.DecryptBytes(encrypted)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *AuthRepository) Create(ctx context.Context, phone string) (model.TelegramAuth, error) {
	encrypted, err := r.cipher.EncryptBytes(nil)
	if err != nil {
		return model.TelegramAuth{}, err
	}
	var item model.TelegramAuth
	err = r.pool.QueryRow(ctx, `
		insert into telegram_accounts (phone_number, session_data, status)
		values ($1, $2, 'authorizing')
		returning id, phone_number, telegram_user_id, telegram_name, telegram_handle,
		          status, last_connected_at, created_at, updated_at
	`, phone, encrypted).Scan(&item.ID, &item.PhoneNumber, &item.TelegramUserID, &item.TelegramName,
		&item.TelegramHandle, &item.Status, &item.LastConnectedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return model.TelegramAuth{}, fmt.Errorf("create telegram account: %w", err)
	}
	return item, nil
}

func (r *AuthRepository) Save(ctx context.Context, auth model.TelegramAuth) error {
	encrypted, err := r.cipher.EncryptBytes(auth.SessionData)
	if err != nil {
		return err
	}

	var current *model.TelegramAuth
	if auth.ID != 0 {
		current, err = r.GetByID(ctx, auth.ID)
	} else {
		current, err = r.Get(ctx)
	}
	if err != nil {
		return err
	}

	if current == nil {
		_, err = r.pool.Exec(ctx, `
			insert into telegram_accounts (
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
		update telegram_accounts
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
	_, err := r.pool.Exec(ctx, `delete from telegram_accounts`)
	if err != nil {
		return fmt.Errorf("clear auth: %w", err)
	}
	return nil
}

func (r *AuthRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `delete from telegram_accounts where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete telegram account %d: %w", id, err)
	}
	return nil
}
