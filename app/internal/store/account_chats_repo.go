package store

import (
	"context"
	"fmt"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountChatRepository struct {
	pool *pgxpool.Pool
}

func (r *AccountChatRepository) Upsert(ctx context.Context, accountID, chatID, accessHash int64) error {
	_, err := r.pool.Exec(ctx, `
		insert into telegram_account_chats (telegram_account_id, chat_id, telegram_access_hash)
		values ($1, $2, $3)
		on conflict (telegram_account_id, chat_id) do update
		set telegram_access_hash = excluded.telegram_access_hash,
		    last_synced_at = now(),
		    updated_at = now()
	`, accountID, chatID, accessHash)
	if err != nil {
		return fmt.Errorf("upsert telegram account chat: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		update chats set collector_account_id = $1, updated_at = now()
		where id = $2 and collector_account_id is null
	`, accountID, chatID)
	if err != nil {
		return fmt.Errorf("set initial collector account: %w", err)
	}
	return nil
}

func (r *AccountChatRepository) ListForChat(ctx context.Context, chatID int64) ([]model.TelegramAccountChat, error) {
	rows, err := r.pool.Query(ctx, `
		select ac.telegram_account_id, ac.chat_id, ac.telegram_access_hash,
		       a.telegram_name, a.telegram_handle, a.status
		from telegram_account_chats ac
		join telegram_accounts a on a.id = ac.telegram_account_id
		where ac.chat_id = $1
		order by a.telegram_name, a.id
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list chat accounts: %w", err)
	}
	defer rows.Close()
	out := make([]model.TelegramAccountChat, 0)
	for rows.Next() {
		var item model.TelegramAccountChat
		if err := rows.Scan(&item.AccountID, &item.ChatID, &item.TelegramAccess, &item.AccountName, &item.AccountHandle, &item.AccountStatus); err != nil {
			return nil, fmt.Errorf("scan chat account: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AccountChatRepository) ListForChats(ctx context.Context, chatIDs []int64) (map[int64][]model.TelegramAccountChat, error) {
	accountsByChat := make(map[int64][]model.TelegramAccountChat, len(chatIDs))
	if len(chatIDs) == 0 {
		return accountsByChat, nil
	}

	rows, err := r.pool.Query(ctx, `
		select ac.telegram_account_id, ac.chat_id, ac.telegram_access_hash,
		       a.telegram_name, a.telegram_handle, a.status
		from telegram_account_chats ac
		join telegram_accounts a on a.id = ac.telegram_account_id
		where ac.chat_id = any($1)
		order by ac.chat_id, a.telegram_name, a.id
	`, chatIDs)
	if err != nil {
		return nil, fmt.Errorf("list telegram account chats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item model.TelegramAccountChat
		if err := rows.Scan(&item.AccountID, &item.ChatID, &item.TelegramAccess, &item.AccountName, &item.AccountHandle, &item.AccountStatus); err != nil {
			return nil, fmt.Errorf("scan telegram account chat: %w", err)
		}
		accountsByChat[item.ChatID] = append(accountsByChat[item.ChatID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate telegram account chats: %w", err)
	}
	return accountsByChat, nil
}

func (r *AccountChatRepository) Get(ctx context.Context, accountID, chatID int64) (model.TelegramAccountChat, error) {
	var item model.TelegramAccountChat
	err := r.pool.QueryRow(ctx, `
		select ac.telegram_account_id, ac.chat_id, ac.telegram_access_hash,
		       a.telegram_name, a.telegram_handle, a.status
		from telegram_account_chats ac
		join telegram_accounts a on a.id = ac.telegram_account_id
		where ac.telegram_account_id = $1 and ac.chat_id = $2
	`, accountID, chatID).Scan(&item.AccountID, &item.ChatID, &item.TelegramAccess, &item.AccountName, &item.AccountHandle, &item.AccountStatus)
	if err != nil {
		return model.TelegramAccountChat{}, fmt.Errorf("get telegram account chat: %w", err)
	}
	return item, nil
}

func (r *AccountChatRepository) CountCollectedBy(ctx context.Context, accountID int64) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `select count(*) from chats where collector_account_id = $1`, accountID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count account collected chats: %w", err)
	}
	return count, nil
}
