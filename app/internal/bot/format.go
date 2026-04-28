package bot

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/frederic/tgtldr/app/internal/model"
)

var (
	mdHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	mdBulletPattern  = regexp.MustCompile(`^\s*[-*+]\s+(.+)$`)
	mdNumberPattern  = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)
	mdLinkPattern    = regexp.MustCompile(`\[(.*?)\]\((https?://[^\s)]+)\)`)
	mdCodePattern    = regexp.MustCompile("`([^`]+)`")
	mdBoldPattern    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	htmlTagPattern   = regexp.MustCompile(`<[^>]+>`)
)

const (
	telegramMessageVisibleLimit = 4096
)

func formatTelegramHTML(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	parts := make([]string, 0, len(lines))
	codeLines := make([]string, 0)
	inCodeBlock := false

	flushCodeBlock := func() {
		if len(codeLines) == 0 {
			parts = append(parts, "<pre></pre>")
			codeLines = codeLines[:0]
			return
		}
		parts = append(parts, "<pre>"+html.EscapeString(strings.Join(codeLines, "\n"))+"</pre>")
		codeLines = codeLines[:0]
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " ")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCodeBlock {
				flushCodeBlock()
				inCodeBlock = false
				continue
			}
			inCodeBlock = true
			codeLines = codeLines[:0]
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, rawLine)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			parts = append(parts, "")
			continue
		}
		if groups := mdHeadingPattern.FindStringSubmatch(trimmed); len(groups) == 3 {
			title := formatInlineTelegramHTML(groups[2])
			if len(groups[1]) <= 2 {
				parts = append(parts, "<b>【"+title+"】</b>")
				continue
			}
			parts = append(parts, "<b>"+title+"</b>")
			continue
		}
		if groups := mdBulletPattern.FindStringSubmatch(trimmed); len(groups) == 2 {
			parts = append(parts, "• "+formatInlineTelegramHTML(groups[1]))
			continue
		}
		if groups := mdNumberPattern.FindStringSubmatch(trimmed); len(groups) == 3 {
			parts = append(parts, fmt.Sprintf("%s. %s", groups[1], formatInlineTelegramHTML(groups[2])))
			continue
		}

		parts = append(parts, formatInlineTelegramHTML(trimmed))
	}

	if inCodeBlock {
		flushCodeBlock()
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func formatInlineTelegramHTML(input string) string {
	placeholders := make([]string, 0)
	withPlaceholders := replacePattern(input, mdLinkPattern, func(groups []string) string {
		return pushPlaceholder(&placeholders, fmt.Sprintf(
			`<a href="%s">%s</a>`,
			html.EscapeString(groups[2]),
			html.EscapeString(groups[1]),
		))
	})
	withPlaceholders = replacePattern(withPlaceholders, mdCodePattern, func(groups []string) string {
		return pushPlaceholder(&placeholders, "<code>"+html.EscapeString(groups[1])+"</code>")
	})
	withPlaceholders = replacePattern(withPlaceholders, mdBoldPattern, func(groups []string) string {
		return pushPlaceholder(&placeholders, "<b>"+html.EscapeString(groups[1])+"</b>")
	})

	escaped := html.EscapeString(withPlaceholders)
	for index, rendered := range placeholders {
		token := placeholderToken(index)
		escaped = strings.ReplaceAll(escaped, html.EscapeString(token), rendered)
	}

	return escaped
}

func formatTelegramMessage(markdown string, language model.Language) string {
	formatted := formatTelegramHTML(markdown)
	truncated, _ := truncateTelegramHTML(formatted, telegramMessageVisibleLimit, language)
	return truncated
}

func telegramVisibleLength(input string) int {
	withoutTags := htmlTagPattern.ReplaceAllString(input, "")
	plain := html.UnescapeString(withoutTags)
	return utf8.RuneCountInString(plain)
}

func truncateTelegramHTML(input string, limit int, language model.Language) (string, bool) {
	if telegramVisibleLength(input) <= limit {
		return input, false
	}

	suffix := "\n\n" + html.EscapeString(telegramTruncationNotice(language))
	contentBudget := limit - telegramVisibleLength(suffix)
	if contentBudget <= 0 {
		return suffix, true
	}

	sections := strings.Split(input, "\n\n")
	kept := make([]string, 0, len(sections))
	used := 0

	for index, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}

		separatorCost := 0
		if len(kept) > 0 {
			separatorCost = 2
		}
		sectionCost := telegramVisibleLength(section)
		if used+separatorCost+sectionCost <= contentBudget {
			kept = append(kept, section)
			used += separatorCost + sectionCost
			continue
		}

		remaining := contentBudget - used - separatorCost
		if remaining > 0 {
			truncatedSection := truncateTelegramHTMLVisible(section, remaining)
			if strings.TrimSpace(truncatedSection) != "" {
				kept = append(kept, truncatedSection)
			}
		}

		if index < len(sections)-1 || len(kept) > 0 {
			return strings.TrimSpace(strings.Join(kept, "\n\n") + suffix), true
		}
		break
	}

	return strings.TrimSpace(strings.Join(kept, "\n\n") + suffix), true
}

func telegramTruncationNotice(language model.Language) string {
	if language == model.LanguageEN {
		return "Note: This Telegram message was truncated because of the single-message length limit. Open the web app to read the full summary."
	}
	return "注：由于 Telegram 单条消息长度限制，后续内容已截断，请到网页端查看完整摘要。"
}

func truncateTelegramHTMLVisible(input string, limit int) string {
	if limit <= 0 {
		return ""
	}

	var builder strings.Builder
	tagStack := make([]string, 0, 8)
	visible := 0

	for index := 0; index < len(input); {
		switch input[index] {
		case '<':
			end := strings.IndexByte(input[index:], '>')
			if end < 0 {
				index = len(input)
				continue
			}
			end += index
			tag := input[index : end+1]
			builder.WriteString(tag)
			updateTelegramTagStack(&tagStack, tag)
			index = end + 1
		case '&':
			end := strings.IndexByte(input[index:], ';')
			if end < 0 {
				r, size := utf8.DecodeRuneInString(input[index:])
				if r == utf8.RuneError && size == 0 {
					index = len(input)
					continue
				}
				if visible >= limit {
					index = len(input)
					continue
				}
				builder.WriteRune(r)
				visible++
				index += size
				continue
			}
			end += index
			entity := input[index : end+1]
			entityVisible := utf8.RuneCountInString(html.UnescapeString(entity))
			if visible+entityVisible > limit {
				index = len(input)
				continue
			}
			builder.WriteString(entity)
			visible += entityVisible
			index = end + 1
		default:
			r, size := utf8.DecodeRuneInString(input[index:])
			if r == utf8.RuneError && size == 0 {
				index = len(input)
				continue
			}
			if visible >= limit {
				index = len(input)
				continue
			}
			builder.WriteRune(r)
			visible++
			index += size
		}
	}

	for i := len(tagStack) - 1; i >= 0; i-- {
		builder.WriteString("</")
		builder.WriteString(tagStack[i])
		builder.WriteByte('>')
	}

	return strings.TrimSpace(builder.String())
}

func updateTelegramTagStack(stack *[]string, tag string) {
	name, closing, ok := parseTelegramTag(tag)
	if !ok {
		return
	}
	if closing {
		for index := len(*stack) - 1; index >= 0; index-- {
			if (*stack)[index] != name {
				continue
			}
			*stack = (*stack)[:index]
			return
		}
		return
	}
	*stack = append(*stack, name)
}

func parseTelegramTag(tag string) (name string, closing bool, ok bool) {
	trimmed := strings.TrimSpace(strings.Trim(tag, "<>"))
	if trimmed == "" {
		return "", false, false
	}
	if strings.HasSuffix(trimmed, "/") {
		return "", false, false
	}
	if strings.HasPrefix(trimmed, "/") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
		if trimmed == "" {
			return "", false, false
		}
		parts := strings.Fields(trimmed)
		return strings.ToLower(parts[0]), true, true
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", false, false
	}
	return strings.ToLower(parts[0]), false, true
}

func replacePattern(input string, pattern *regexp.Regexp, render func([]string) string) string {
	matches := pattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input
	}

	var builder strings.Builder
	lastIndex := 0
	for _, match := range matches {
		builder.WriteString(input[lastIndex:match[0]])
		groups := make([]string, 0, len(match)/2)
		for i := 0; i < len(match); i += 2 {
			if match[i] < 0 || match[i+1] < 0 {
				groups = append(groups, "")
				continue
			}
			groups = append(groups, input[match[i]:match[i+1]])
		}
		builder.WriteString(render(groups))
		lastIndex = match[1]
	}
	builder.WriteString(input[lastIndex:])
	return builder.String()
}

func pushPlaceholder(placeholders *[]string, rendered string) string {
	index := len(*placeholders)
	*placeholders = append(*placeholders, rendered)
	return placeholderToken(index)
}

func placeholderToken(index int) string {
	return fmt.Sprintf("%%TGTLDR_HTML_%d%%", index)
}
