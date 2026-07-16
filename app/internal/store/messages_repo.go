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

type MessageCursor struct {
	MessageTime       time.Time
	TelegramMessageID int
}

type MessageEntityCandidate struct {
	MessageID          int64
	ChatID             int64
	TelegramMessageID  int
	TelegramChatID     int64
	TelegramAccessHash int64
	ChatType           string
}

// GetMediaMessageCandidate 返回刷新单条消息媒体引用所需的群组定位信息。
func (r *MessageRepository) GetMediaMessageCandidate(ctx context.Context, accountID int64, messageID int64) (MessageEntityCandidate, error) {
	var item MessageEntityCandidate
	err := r.pool.QueryRow(ctx, `
		select m.id, m.chat_id, m.telegram_message_id, c.telegram_chat_id,
		       coalesce(tac.telegram_access_hash, c.telegram_access_hash), c.chat_type
		from messages m
		join chats c on c.id = m.chat_id
		left join telegram_account_chats tac
		  on tac.chat_id = c.id and tac.telegram_account_id = $1
		where m.id = $2 and c.collector_account_id = $1
	`, accountID, messageID).Scan(&item.MessageID, &item.ChatID, &item.TelegramMessageID,
		&item.TelegramChatID, &item.TelegramAccessHash, &item.ChatType)
	if err != nil {
		return MessageEntityCandidate{}, fmt.Errorf("get media message candidate %d: %w", messageID, err)
	}
	return item, nil
}

// ListMissingSenderEntities 返回升级前尚未关联 Telegram 实体的消息。
func (r *MessageRepository) ListMissingSenderEntities(ctx context.Context, accountID int64, afterID int64, limit int) ([]MessageEntityCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		select m.id, m.chat_id, m.telegram_message_id, c.telegram_chat_id,
		       coalesce(tac.telegram_access_hash, c.telegram_access_hash), c.chat_type
		from messages m
		join chats c on c.id = m.chat_id
		left join telegram_account_chats tac
		  on tac.chat_id = c.id and tac.telegram_account_id = $1
		where c.collector_account_id = $1 and m.sender_entity_id is null
		  and m.telegram_sender_id <> 0 and m.id > $2
		order by m.id asc limit $3
	`, accountID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list missing sender entities: %w", err)
	}
	defer rows.Close()
	items := make([]MessageEntityCandidate, 0, limit)
	for rows.Next() {
		var item MessageEntityCandidate
		if err := rows.Scan(&item.MessageID, &item.ChatID, &item.TelegramMessageID,
			&item.TelegramChatID, &item.TelegramAccessHash, &item.ChatType); err != nil {
			return nil, fmt.Errorf("scan missing sender entity: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// AttachSenderEntity 关联补齐的发言人实体，并刷新原消息中的 Telegram 文件引用。
func (r *MessageRepository) AttachSenderEntity(ctx context.Context, messageID int64, entityID int64, rawJSON string) error {
	_, err := r.pool.Exec(ctx, `
		update messages set sender_entity_id = $2, raw_json = $3::jsonb where id = $1
	`, messageID, entityID, rawJSON)
	if err != nil {
		return fmt.Errorf("attach sender entity to message %d: %w", messageID, err)
	}
	return nil
}

// ListMediaAfter 按本地消息 ID 扫描账号已有媒体，供升级后的后台补齐使用。
func (r *MessageRepository) ListMediaAfter(ctx context.Context, accountID int64, afterID int64, limit int) ([]model.Message, error) {
	rows, err := r.pool.Query(ctx, `
		select m.id, m.chat_id, coalesce(m.sender_entity_id, 0), m.telegram_message_id,
		       m.telegram_sender_id, m.sender_name, m.sender_username, m.sender_is_bot,
		       m.text_content, m.caption, m.message_type, m.media_kind, m.reply_to_message_id,
		       m.message_time, m.raw_json::text, m.created_at
		from messages m
		join chats c on c.id = m.chat_id
		where c.collector_account_id = $1 and m.id > $2 and m.media_kind in ('photo', 'document')
		order by m.id asc limit $3
	`, accountID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list stored media after %d: %w", afterID, err)
	}
	defer rows.Close()
	messages := make([]model.Message, 0, limit)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *MessageRepository) Upsert(ctx context.Context, message model.Message) error {
	_, err := r.pool.Exec(ctx, `
		insert into messages (
			chat_id, sender_entity_id, telegram_message_id, telegram_sender_id, sender_name, sender_username, sender_is_bot,
			text_content, caption, message_type, media_kind, reply_to_message_id,
			message_time, raw_json
		) values ($1, nullif($2, 0), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb)
		on conflict (chat_id, telegram_message_id) do update
		set sender_entity_id = coalesce(excluded.sender_entity_id, messages.sender_entity_id),
		    telegram_sender_id = excluded.telegram_sender_id,
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
		message.SenderEntityID,
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
		select id, chat_id, coalesce(sender_entity_id, 0), telegram_message_id, telegram_sender_id, sender_name,
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
		var message model.Message
		err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.SenderEntityID,
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
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *MessageRepository) ListPageForRange(
	ctx context.Context,
	chatID int64,
	start time.Time,
	end time.Time,
	before *MessageCursor,
	limit int,
	filter MessageDisplayFilter,
) ([]model.Message, bool, error) {
	query := `
		select id, chat_id, coalesce(sender_entity_id, 0), telegram_message_id, telegram_sender_id, sender_name,
		       sender_username, sender_is_bot,
		       text_content, caption, message_type, media_kind, reply_to_message_id,
		       message_time, raw_json::text, created_at
		from messages m
		where m.chat_id = $1 and m.message_time >= $2 and m.message_time < $3`
	args := []any{chatID, start, end}
	if before != nil {
		query += ` and (m.message_time, m.telegram_message_id) < ($4, $5)`
		args = append(args, before.MessageTime, before.TelegramMessageID)
	}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	query += filterSQL
	query += ` order by m.message_time desc, m.telegram_message_id desc limit $` + fmt.Sprint(len(args)+1)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query message page: %w", err)
	}
	defer rows.Close()

	messages := make([]model.Message, 0, limit+1)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, false, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate message page: %w", err)
	}

	messages, hasMore := normalizeMessagePage(messages, limit)
	return messages, hasMore, nil
}

func (r *MessageRepository) LatestDateAtOrBefore(
	ctx context.Context,
	chatID int64,
	timezone string,
	date string,
	filter MessageDisplayFilter,
) (string, error) {
	args := []any{chatID, timezone, date}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	var latest *time.Time
	err := r.pool.QueryRow(ctx, `
		select max((m.message_time at time zone $2)::date)
		from messages m
		where m.chat_id = $1 and (m.message_time at time zone $2)::date <= $3::date`+
		filterSQL, args...).Scan(&latest)
	if err != nil {
		return "", fmt.Errorf("query latest message date: %w", err)
	}
	if latest == nil {
		return "", nil
	}
	return latest.Format("2006-01-02"), nil
}

func (r *MessageRepository) AdjacentDates(
	ctx context.Context,
	chatID int64,
	timezone string,
	date string,
	filter MessageDisplayFilter,
) (string, string, error) {
	args := []any{chatID, timezone, date}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	var previous *time.Time
	var next *time.Time
	err := r.pool.QueryRow(ctx, `
		select
			max((m.message_time at time zone $2)::date)
				filter (where (m.message_time at time zone $2)::date < $3::date),
			min((m.message_time at time zone $2)::date)
				filter (where (m.message_time at time zone $2)::date > $3::date)
		from messages m
		where m.chat_id = $1`+filterSQL, args...).Scan(&previous, &next)
	if err != nil {
		return "", "", fmt.Errorf("query adjacent message dates: %w", err)
	}
	return formatOptionalDate(previous), formatOptionalDate(next), nil
}

func (r *MessageRepository) ActivityForRange(
	ctx context.Context,
	chatID int64,
	startLocal time.Time,
	days int,
	timezone string,
	filter MessageDisplayFilter,
) ([]model.ChatMessageActivity, error) {
	endLocal := startLocal.AddDate(0, 0, days)
	args := []any{chatID, startLocal.UTC(), endLocal.UTC(), timezone}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	rows, err := r.pool.Query(ctx, `
		select (m.message_time at time zone $4)::date::text, count(*)::int
		from messages m
		where m.chat_id = $1 and m.message_time >= $2 and m.message_time < $3`+filterSQL+`
		group by 1
		order by 1
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query message activity: %w", err)
	}
	defer rows.Close()

	sparse := make([]model.ChatMessageActivity, 0, days)
	for rows.Next() {
		var item model.ChatMessageActivity
		if err := rows.Scan(&item.Date, &item.MessageCount); err != nil {
			return nil, fmt.Errorf("scan message activity: %w", err)
		}
		sparse = append(sparse, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message activity: %w", err)
	}
	return completeMessageActivity(startLocal, days, sparse), nil
}

func (r *MessageRepository) CountForRange(
	ctx context.Context,
	chatID int64,
	start time.Time,
	end time.Time,
	filters ...MessageDisplayFilter,
) (int, error) {
	filter := MessageDisplayFilter{}
	if len(filters) > 0 {
		filter = filters[0]
	}
	args := []any{chatID, start, end}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	var count int
	err := r.pool.QueryRow(ctx, `
		select count(*)
		from messages m
		where m.chat_id = $1 and m.message_time >= $2 and m.message_time < $3`+
		filterSQL, args...).Scan(&count)
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
		select id, chat_id, coalesce(sender_entity_id, 0), telegram_message_id, telegram_sender_id, sender_name,
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
		var message model.Message
		err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.SenderEntityID,
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
		if err != nil {
			return nil, fmt.Errorf("scan lookup message: %w", err)
		}
		lookup[message.TelegramMessageID] = message
	}
	return lookup, rows.Err()
}

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessage(scanner messageScanner) (model.Message, error) {
	var message model.Message
	err := scanner.Scan(
		&message.ID,
		&message.ChatID,
		&message.SenderEntityID,
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
	if err != nil {
		return model.Message{}, fmt.Errorf("scan message: %w", err)
	}
	return message, nil
}

func reverseMessages(messages []model.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

func normalizeMessagePage(messages []model.Message, limit int) ([]model.Message, bool) {
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	reverseMessages(messages)
	return messages, hasMore
}

func formatOptionalDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}
