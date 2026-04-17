package bot

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	mdHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	mdBulletPattern  = regexp.MustCompile(`^\s*[-*+]\s+(.+)$`)
	mdNumberPattern  = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)
	mdLinkPattern    = regexp.MustCompile(`\[(.*?)\]\((https?://[^\s)]+)\)`)
	mdCodePattern    = regexp.MustCompile("`([^`]+)`")
	mdBoldPattern    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
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
