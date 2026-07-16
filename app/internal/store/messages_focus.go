package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
)

func (r *MessageRepository) ListWindowAround(
	ctx context.Context,
	chatID int64,
	start time.Time,
	end time.Time,
	messageID int64,
	beforeCount int,
	afterCount int,
	filter MessageDisplayFilter,
) ([]model.Message, bool, error) {
	target, err := r.getMessageInRange(ctx, chatID, start, end, messageID, filter)
	if err != nil {
		return nil, false, err
	}

	before, err := r.listMessagesBeforeTarget(
		ctx, chatID, start, end, target, beforeCount+2, filter,
	)
	if err != nil {
		return nil, false, err
	}
	hasMoreBefore := len(before) > beforeCount+1
	if hasMoreBefore {
		before = before[:beforeCount+1]
	}
	reverseMessages(before)

	after, err := r.listMessagesAfterTarget(
		ctx, chatID, start, end, target, afterCount, filter,
	)
	if err != nil {
		return nil, false, err
	}
	return append(before, after...), hasMoreBefore, nil
}

func (r *MessageRepository) getMessageInRange(
	ctx context.Context,
	chatID int64,
	start time.Time,
	end time.Time,
	messageID int64,
	filter MessageDisplayFilter,
) (model.Message, error) {
	args := []any{chatID, start, end, messageID}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	row := r.pool.QueryRow(ctx, messageSelectSQL+`
		where m.chat_id = $1 and m.message_time >= $2 and m.message_time < $3 and m.id = $4`+
		filterSQL, args...)
	message, err := scanMessage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Message{}, err
		}
		return model.Message{}, fmt.Errorf("get focused message: %w", err)
	}
	return message, nil
}

func (r *MessageRepository) listMessagesBeforeTarget(
	ctx context.Context,
	chatID int64,
	start time.Time,
	end time.Time,
	target model.Message,
	limit int,
	filter MessageDisplayFilter,
) ([]model.Message, error) {
	args := []any{chatID, start, end, target.MessageTime, target.TelegramMessageID}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	args = append(args, limit)
	return r.queryMessageWindow(ctx, messageSelectSQL+`
		where m.chat_id = $1 and m.message_time >= $2 and m.message_time < $3
		  and (m.message_time, m.telegram_message_id) <= ($4, $5)`+filterSQL+`
		order by m.message_time desc, m.telegram_message_id desc
		limit $`+fmt.Sprint(len(args))+`
	`, args...)
}

func (r *MessageRepository) listMessagesAfterTarget(
	ctx context.Context,
	chatID int64,
	start time.Time,
	end time.Time,
	target model.Message,
	limit int,
	filter MessageDisplayFilter,
) ([]model.Message, error) {
	args := []any{chatID, start, end, target.MessageTime, target.TelegramMessageID}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	args = append(args, limit)
	return r.queryMessageWindow(ctx, messageSelectSQL+`
		where m.chat_id = $1 and m.message_time >= $2 and m.message_time < $3
		  and (m.message_time, m.telegram_message_id) > ($4, $5)`+filterSQL+`
		order by m.message_time asc, m.telegram_message_id asc
		limit $`+fmt.Sprint(len(args))+`
	`, args...)
}

func (r *MessageRepository) queryMessageWindow(
	ctx context.Context,
	query string,
	args ...any,
) ([]model.Message, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query focused message window: %w", err)
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate focused message window: %w", err)
	}
	return messages, nil
}

const messageSelectSQL = `
	select id, chat_id, coalesce(sender_entity_id, 0), telegram_message_id, telegram_sender_id, sender_name,
	       sender_username, sender_is_bot,
	       text_content, caption, message_type, media_kind, reply_to_message_id,
	       message_time, raw_json::text, created_at
	from messages m
`
