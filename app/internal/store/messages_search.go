package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

const (
	defaultMessageSearchPage     = 1
	defaultMessageSearchPageSize = 50
	maxMessageSearchPageSize     = 100
)

type MessageSearchParams struct {
	ChatID   int64
	Query    string
	Timezone string
	Page     int
	PageSize int
	Filter   MessageDisplayFilter
}

type MessageSearchItem struct {
	Message       model.Message
	LocalDate     string
	MatchSnippet  string
	MatchedFields []string
}

type MessageSearchResult struct {
	Items    []MessageSearchItem
	Total    int
	Page     int
	PageSize int
}

func (r *MessageRepository) Search(
	ctx context.Context,
	params MessageSearchParams,
) (MessageSearchResult, error) {
	params = normalizeMessageSearchParams(params)
	terms := strings.Fields(params.Query)
	where, args := buildMessageSearchWhere(params.ChatID, terms, params.Filter)

	var total int
	countQuery := `select count(*) from messages m` + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return MessageSearchResult{}, fmt.Errorf("count searched messages: %w", err)
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Timezone, params.PageSize, (params.Page-1)*params.PageSize)
	timezoneIndex := len(args) + 1
	limitIndex := timezoneIndex + 1
	offsetIndex := limitIndex + 1
	query := fmt.Sprintf(`
		select id, chat_id, telegram_message_id, telegram_sender_id, sender_name,
		       sender_username, sender_is_bot,
		       text_content, caption, message_type, media_kind, reply_to_message_id,
		       message_time, raw_json::text, created_at,
		       (message_time at time zone $%d)::date::text
		from messages m%s
		order by message_time desc, telegram_message_id desc
		limit $%d offset $%d
	`, timezoneIndex, where, limitIndex, offsetIndex)

	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return MessageSearchResult{}, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	items := make([]MessageSearchItem, 0, params.PageSize)
	for rows.Next() {
		var item MessageSearchItem
		message, err := scanSearchMessage(rows, &item.LocalDate)
		if err != nil {
			return MessageSearchResult{}, err
		}
		item.Message = message
		item.MatchSnippet, item.MatchedFields = messageSearchMatch(message, terms)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MessageSearchResult{}, fmt.Errorf("iterate searched messages: %w", err)
	}
	return MessageSearchResult{
		Items: items, Total: total, Page: params.Page, PageSize: params.PageSize,
	}, nil
}

func normalizeMessageSearchParams(params MessageSearchParams) MessageSearchParams {
	params.Query = strings.TrimSpace(params.Query)
	if params.Page < 1 {
		params.Page = defaultMessageSearchPage
	}
	if params.PageSize < 1 {
		params.PageSize = defaultMessageSearchPageSize
	}
	if params.PageSize > maxMessageSearchPageSize {
		params.PageSize = maxMessageSearchPageSize
	}
	return params
}

func buildMessageSearchWhere(
	chatID int64,
	terms []string,
	filter MessageDisplayFilter,
) (string, []any) {
	args := []any{chatID}
	clauses := []string{"m.chat_id = $1"}
	searchDocument := "(m.text_content || ' ' || m.caption || ' ' || m.sender_name || ' ' || m.sender_username)"
	for _, term := range terms {
		args = append(args, "%"+term+"%")
		clauses = append(clauses, fmt.Sprintf("%s ilike $%d", searchDocument, len(args)))
	}
	filterSQL, args := appendMessageDisplayFilter("m", args, filter)
	return " where " + strings.Join(clauses, " and ") + filterSQL, args
}

func scanSearchMessage(scanner messageScanner, localDate *string) (model.Message, error) {
	var message model.Message
	err := scanner.Scan(
		&message.ID, &message.ChatID, &message.TelegramMessageID,
		&message.TelegramSenderID, &message.SenderName, &message.SenderUsername,
		&message.SenderIsBot, &message.TextContent, &message.Caption,
		&message.MessageType, &message.MediaKind, &message.ReplyToMessageID,
		&message.MessageTime, &message.RawJSON, &message.CreatedAt, localDate,
	)
	if err != nil {
		return model.Message{}, fmt.Errorf("scan searched message: %w", err)
	}
	return message, nil
}

func messageSearchMatch(message model.Message, terms []string) (string, []string) {
	content := collapseWhitespace(message.SummaryText())
	sender := strings.TrimSpace(message.SenderName + " @" + message.SenderUsername)
	matchedFields := make([]string, 0, 2)
	if matchesAny(content, terms) {
		matchedFields = append(matchedFields, "content")
	}
	if matchesAny(sender, terms) {
		matchedFields = append(matchedFields, "sender")
	}
	if content != "" {
		return buildMatchSnippet(content, terms), matchedFields
	}
	return sender, matchedFields
}

func matchesAny(value string, terms []string) bool {
	for _, term := range terms {
		if containsFold(value, term) {
			return true
		}
	}
	return false
}
