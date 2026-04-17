package summary

import (
	"fmt"
	"strings"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
)

type Chunk struct {
	Index    int
	Messages []model.Message
}

func BuildTranscript(messages []model.Message, lookup map[int]model.Message, location *time.Location) string {
	if len(messages) == 0 {
		return ""
	}

	localRefs := make(map[int]string, len(messages))
	for index, message := range messages {
		localRefs[message.TelegramMessageID] = fmt.Sprintf("m%03d", index+1)
	}

	externalRefs := make(map[int]string)
	externalOrder := make([]int, 0)
	blocks := make([]string, 0, len(messages))

	for _, message := range messages {
		text := strings.TrimSpace(message.SummaryText())
		if text == "" {
			continue
		}

		blockLines := []string{
			fmt.Sprintf("[%s] %s %s", localRefs[message.TelegramMessageID], formatTranscriptTime(message.MessageTime, location), fallback(message.SenderName, "Unknown")),
		}

		if message.ReplyToMessageID > 0 {
			replyRef, replyExcerpt := resolveReplyReference(message.ReplyToMessageID, localRefs, lookup, externalRefs, &externalOrder)
			if replyRef != "" {
				blockLines = append(blockLines, fmt.Sprintf("reply_to=[%s]", replyRef))
			}
			if replyExcerpt != "" {
				blockLines = append(blockLines, fmt.Sprintf("reply_excerpt=%q", replyExcerpt))
			}
		}

		blockLines = append(blockLines, text)
		blocks = append(blocks, strings.Join(blockLines, "\n"))
	}

	sections := make([]string, 0, 2)
	if len(externalOrder) > 0 {
		referenced := make([]string, 0, len(externalOrder)+1)
		referenced = append(referenced, "[Referenced Messages]")
		for _, messageID := range externalOrder {
			reference := lookup[messageID]
			label := externalRefs[messageID]
			referenced = append(
				referenced,
				fmt.Sprintf("[%s] %s %s", label, formatTranscriptTime(reference.MessageTime, location), fallback(reference.SenderName, "Unknown")),
				referenceSummaryText(reference),
			)
		}
		sections = append(sections, strings.Join(referenced, "\n"))
	}

	sections = append(sections, "[Messages]\n"+strings.Join(blocks, "\n\n"))
	return strings.Join(sections, "\n\n")
}

func formatTranscriptTime(messageTime time.Time, location *time.Location) string {
	if location == nil {
		return messageTime.Format("15:04")
	}
	return messageTime.In(location).Format("15:04")
}

func SplitMessages(messages []model.Message, maxTokens int) []Chunk {
	if len(messages) == 0 {
		return nil
	}

	const (
		preferredGap           = 90 * time.Minute
		minGapSplitFillPercent = 0.35
	)
	var chunks []Chunk
	current := make([]model.Message, 0, 64)
	currentTokens := 0
	chunkIndex := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		cloned := make([]model.Message, len(current))
		copy(cloned, current)
		chunks = append(chunks, Chunk{Index: chunkIndex, Messages: cloned})
		chunkIndex++
		current = current[:0]
		currentTokens = 0
	}

	for idx, message := range messages {
		messageTokens := estimateTokens(message.SummaryText())
		if messageTokens == 0 {
			messageTokens = 10
		}

		if idx > 0 {
			gap := message.MessageTime.Sub(messages[idx-1].MessageTime)
			minGapSplitTokens := int(float64(maxTokens) * minGapSplitFillPercent)
			if gap >= preferredGap && len(current) > 0 && currentTokens >= minGapSplitTokens {
				flush()
			}
		}

		if len(current) > 0 && currentTokens+messageTokens > maxTokens {
			flush()
		}

		current = append(current, message)
		currentTokens += messageTokens
	}

	flush()
	return chunks
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return len([]rune(trimmed))/4 + 16
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func resolveReplyReference(
	replyToMessageID int,
	localRefs map[int]string,
	lookup map[int]model.Message,
	externalRefs map[int]string,
	externalOrder *[]int,
) (string, string) {
	if localRef, ok := localRefs[replyToMessageID]; ok {
		reference := lookup[replyToMessageID]
		return localRef, compactReplyExcerpt(referenceSummaryText(reference))
	}

	reference, ok := lookup[replyToMessageID]
	if !ok {
		return fmt.Sprintf("msg:%d", replyToMessageID), "[原始消息未在当前数据库中找到]"
	}

	if externalRef, ok := externalRefs[replyToMessageID]; ok {
		return externalRef, compactReplyExcerpt(referenceSummaryText(reference))
	}

	externalRef := fmt.Sprintf("ref%03d", len(externalRefs)+1)
	externalRefs[replyToMessageID] = externalRef
	*externalOrder = append(*externalOrder, replyToMessageID)
	return externalRef, compactReplyExcerpt(referenceSummaryText(reference))
}

func compactReplyExcerpt(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	normalized := strings.Join(strings.Fields(trimmed), " ")
	runes := []rune(normalized)
	if len(runes) <= 96 {
		return normalized
	}
	return string(runes[:96]) + "…"
}

func referenceSummaryText(message model.Message) string {
	if text := strings.TrimSpace(message.SummaryText()); text != "" {
		return text
	}

	switch strings.TrimSpace(message.MediaKind) {
	case "photo":
		return "[图片消息，无文字说明]"
	case "document":
		return "[文件消息，无文字说明]"
	}

	if strings.TrimSpace(message.MessageType) != "" && strings.TrimSpace(message.MessageType) != "text" {
		return "[非文本消息，无文字说明]"
	}

	return "[无可读文本内容]"
}
