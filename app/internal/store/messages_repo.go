package store

import (
	"context"
	"fmt"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	pool *pgxpool.Pool
}

type MessageParticipant struct {
	TelegramSenderID int64
	SenderName       string
	SenderUsername   string
	MessageCount     int64
	FirstMessageTime time.Time
	LastMessageTime  time.Time
}

func (r *MessageRepository) Upsert(ctx context.Context, message model.Message) error {
	_, err := r.pool.Exec(ctx, `
		insert into messages (
			chat_id, telegram_message_id, telegram_sender_id, sender_name, sender_username, sender_is_bot,
			text_content, caption, message_type, media_kind, reply_to_message_id,
			message_time, raw_json
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		on conflict (chat_id, telegram_message_id) do update
		set telegram_sender_id = excluded.telegram_sender_id,
		    sender_name = excluded.sender_name,
		    sender_username = excluded.sender_username,
		    sender_is_bot = excluded.sender_is_bot,
		    text_content = excluded.text_content,
		    caption = excluded.caption,
		    message_type = excluded.message_type,
		    media_kind = excluded.media_kind,
		    reply_to_message_id = excluded.reply_to_message_id,
		    message_time = excluded.message_time,
		    raw_json = excluded.raw_json
	`,
		message.ChatID,
		message.TelegramMessageID,
		message.TelegramSenderID,
		message.SenderName,
		message.SenderUsername,
		message.SenderIsBot,
		message.TextContent,
		message.Caption,
		message.MessageType,
		message.MediaKind,
		message.ReplyToMessageID,
		message.MessageTime,
		message.RawJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert message %d: %w", message.TelegramMessageID, err)
	}
	return nil
}

func (r *MessageRepository) ListForRange(ctx context.Context, chatID int64, start, end time.Time) ([]model.Message, error) {
	rows, err := r.pool.Query(ctx, `
		select id, chat_id, telegram_message_id, telegram_sender_id, sender_name,
		       sender_username, sender_is_bot,
		       text_content, caption, message_type, media_kind, reply_to_message_id,
		       message_time, raw_json::text, created_at
		from messages
		where chat_id = $1 and message_time >= $2 and message_time < $3
		order by message_time asc, telegram_message_id asc
	`, chatID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []model.Message
	for rows.Next() {
		message, err := scanStoredMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *MessageRepository) CountForRange(ctx context.Context, chatID int64, start, end time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		select count(*)
		from messages
		where chat_id = $1 and message_time >= $2 and message_time < $3
	`, chatID, start, end).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return count, nil
}

func (r *MessageRepository) LookupByTelegramIDs(ctx context.Context, chatID int64, ids []int) (map[int]model.Message, error) {
	if len(ids) == 0 {
		return map[int]model.Message{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		select id, chat_id, telegram_message_id, telegram_sender_id, sender_name,
		       sender_username, sender_is_bot,
		       text_content, caption, message_type, media_kind, reply_to_message_id,
		       message_time, raw_json::text, created_at
		from messages
		where chat_id = $1 and telegram_message_id = any($2)
	`, chatID, ids)
	if err != nil {
		return nil, fmt.Errorf("lookup messages by telegram ids: %w", err)
	}
	defer rows.Close()

	lookup := make(map[int]model.Message, len(ids))
	for rows.Next() {
		message, err := scanStoredMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lookup message: %w", err)
		}
		lookup[message.TelegramMessageID] = message
	}
	return lookup, rows.Err()
}

func (r *MessageRepository) SearchParticipants(ctx context.Context, chatID int64, query string, limit int) ([]MessageParticipant, error) {
	rows, err := r.pool.Query(ctx, `
		with matched_senders as (
			select distinct telegram_sender_id
			from messages
			where chat_id = $1
			  and telegram_sender_id > 0
			  and (lower(sender_username) = lower($2) or strpos(lower(sender_name), lower($2)) > 0)
		), stats as (
			select m.telegram_sender_id, count(*) as message_count,
			       min(m.message_time) as first_message_time,
			       max(m.message_time) as last_message_time
			from messages m
			join matched_senders matched using (telegram_sender_id)
			where m.chat_id = $1
			group by m.telegram_sender_id
		), latest as (
			select distinct on (m.telegram_sender_id)
			       m.telegram_sender_id, m.sender_name, m.sender_username
			from messages m
			join matched_senders matched using (telegram_sender_id)
			where m.chat_id = $1
			order by m.telegram_sender_id, m.message_time desc, m.telegram_message_id desc
		)
		select stats.telegram_sender_id, latest.sender_name, latest.sender_username,
		       stats.message_count, stats.first_message_time, stats.last_message_time
		from stats
		join latest using (telegram_sender_id)
		order by (lower(latest.sender_username) = lower($2)) desc, stats.last_message_time desc
		limit $3
	`, chatID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search message participants: %w", err)
	}
	defer rows.Close()

	participants := make([]MessageParticipant, 0)
	for rows.Next() {
		var participant MessageParticipant
		if err := rows.Scan(
			&participant.TelegramSenderID,
			&participant.SenderName,
			&participant.SenderUsername,
			&participant.MessageCount,
			&participant.FirstMessageTime,
			&participant.LastMessageTime,
		); err != nil {
			return nil, fmt.Errorf("scan message participant: %w", err)
		}
		participants = append(participants, participant)
	}
	return participants, rows.Err()
}

func (r *MessageRepository) ListMessages(
	ctx context.Context,
	chatID int64,
	senderID *int64,
	from *time.Time,
	to *time.Time,
	cursorTime *time.Time,
	cursorMessageID int,
	limit int,
) ([]model.Message, error) {
	rows, err := r.pool.Query(ctx, `
		select telegram_message_id, telegram_sender_id, sender_name, sender_username,
		       text_content, caption, message_type, media_kind, reply_to_message_id,
		       message_time
		from messages
		where chat_id = $1
		  and ($2::bigint is null or telegram_sender_id = $2)
		  and ($3::timestamptz is null or message_time >= $3)
		  and ($4::timestamptz is null or message_time < $4)
		  and ($5::timestamptz is null or (message_time, telegram_message_id) > ($5, $6))
		order by message_time asc, telegram_message_id asc
		limit $7
	`, chatID, senderID, from, to, cursorTime, cursorMessageID, limit)
	if err != nil {
		return nil, fmt.Errorf("query messages by sender: %w", err)
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		message, err := scanReadAPIMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sender message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

type messageScanner interface {
	Scan(dest ...any) error
}

func scanStoredMessage(scanner messageScanner) (model.Message, error) {
	var message model.Message
	err := scanner.Scan(
		&message.ID,
		&message.ChatID,
		&message.TelegramMessageID,
		&message.TelegramSenderID,
		&message.SenderName,
		&message.SenderUsername,
		&message.SenderIsBot,
		&message.TextContent,
		&message.Caption,
		&message.MessageType,
		&message.MediaKind,
		&message.ReplyToMessageID,
		&message.MessageTime,
		&message.RawJSON,
		&message.CreatedAt,
	)
	return message, err
}

func scanReadAPIMessage(scanner messageScanner) (model.Message, error) {
	var message model.Message
	err := scanner.Scan(
		&message.TelegramMessageID,
		&message.TelegramSenderID,
		&message.SenderName,
		&message.SenderUsername,
		&message.TextContent,
		&message.Caption,
		&message.MessageType,
		&message.MediaKind,
		&message.ReplyToMessageID,
		&message.MessageTime,
	)
	return message, err
}
