package store

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	defaultSummaryPage     = 1
	defaultSummaryPageSize = 20
	maxSummaryPageSize     = 100
	snippetContextRunes    = 36
)

var markdownLinkPattern = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)

type SummaryListParams struct {
	Query    string
	ChatID   int64
	Status   string
	Delivery string
	DateFrom string
	DateTo   string
	Page     int
	PageSize int
}

func normalizeSummaryListParams(params SummaryListParams) SummaryListParams {
	if params.Page < 1 {
		params.Page = defaultSummaryPage
	}
	if params.PageSize < 1 {
		params.PageSize = defaultSummaryPageSize
	}
	if params.PageSize > maxSummaryPageSize {
		params.PageSize = maxSummaryPageSize
	}
	params.Query = strings.TrimSpace(params.Query)
	params.Status = strings.TrimSpace(params.Status)
	params.Delivery = strings.TrimSpace(params.Delivery)
	params.DateFrom = strings.TrimSpace(params.DateFrom)
	params.DateTo = strings.TrimSpace(params.DateTo)
	return params
}

func buildSummaryWhereClause(params SummaryListParams, terms []string) (string, []any) {
	clauses := make([]string, 0)
	args := make([]any, 0)

	if params.ChatID > 0 {
		args = append(args, params.ChatID)
		clauses = append(clauses, fmt.Sprintf("s.chat_id = $%d", len(args)))
	}
	if params.Status != "" && params.Status != "all" {
		args = append(args, params.Status)
		clauses = append(clauses, fmt.Sprintf("s.status = $%d", len(args)))
	}
	if params.DateFrom != "" {
		args = append(args, params.DateFrom)
		clauses = append(clauses, fmt.Sprintf("s.summary_date >= $%d::date", len(args)))
	}
	if params.DateTo != "" {
		args = append(args, params.DateTo)
		clauses = append(clauses, fmt.Sprintf("s.summary_date <= $%d::date", len(args)))
	}
	if deliveryClause := buildDeliveryClause(params.Delivery); deliveryClause != "" {
		clauses = append(clauses, deliveryClause)
	}
	for _, term := range terms {
		args = append(args, "%"+term+"%")
		index := len(args)
		clauses = append(clauses, fmt.Sprintf("(s.content ilike $%d or c.title ilike $%d)", index, index))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " where " + strings.Join(clauses, " and "), args
}

func buildDeliveryClause(delivery string) string {
	switch delivery {
	case "", "all":
		return ""
	case "sent":
		return "s.delivered_at is not null"
	case "pending":
		return "c.delivery_mode = 'bot' and s.delivered_at is null and s.delivery_error = ''"
	case "failed":
		return "c.delivery_mode = 'bot' and s.delivered_at is null and s.delivery_error <> ''"
	case "disabled":
		return "c.delivery_mode <> 'bot'"
	default:
		return ""
	}
}

func searchTerms(query string) []string {
	if query == "" {
		return nil
	}
	return strings.Fields(query)
}

func summarizeSearchMatch(content string, chatTitle string, terms []string) (string, []string) {
	if len(terms) == 0 {
		return "", nil
	}

	plainContent := plainSearchText(content)
	matchedFields := make([]string, 0, 2)
	contentMatched := false
	titleMatched := false
	for _, term := range terms {
		if !contentMatched && containsFold(plainContent, term) {
			contentMatched = true
		}
		if !titleMatched && containsFold(chatTitle, term) {
			titleMatched = true
		}
	}
	if contentMatched {
		matchedFields = append(matchedFields, "content")
	}
	if titleMatched {
		matchedFields = append(matchedFields, "title")
	}

	if contentMatched {
		return buildMatchSnippet(plainContent, terms), matchedFields
	}
	if titleMatched {
		return "匹配到群组名称", matchedFields
	}
	return "", matchedFields
}

func plainSearchText(value string) string {
	if value == "" {
		return ""
	}

	lines := strings.Split(markdownLinkPattern.ReplaceAllString(value, "$1"), "\n")
	cleaned := make([]string, 0, len(lines))
	replacer := strings.NewReplacer("**", "", "__", "", "`", "")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimLeft(trimmed, "#")
		trimmed = strings.TrimSpace(trimmed)
		switch {
		case strings.HasPrefix(trimmed, "- "),
			strings.HasPrefix(trimmed, "* "),
			strings.HasPrefix(trimmed, "• "),
			strings.HasPrefix(trimmed, "> "):
			trimmed = strings.TrimSpace(trimmed[2:])
		}
		trimmed = replacer.Replace(trimmed)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return collapseWhitespace(strings.Join(cleaned, " "))
}

func containsFold(content string, term string) bool {
	return strings.Contains(strings.ToLower(content), strings.ToLower(term))
}

func buildMatchSnippet(content string, terms []string) string {
	normalized := collapseWhitespace(content)
	if normalized == "" {
		return ""
	}

	matchIndex := -1
	matchLength := 0
	lower := strings.ToLower(normalized)
	for _, term := range terms {
		index := strings.Index(lower, strings.ToLower(term))
		if index == -1 {
			continue
		}
		if matchIndex == -1 || index < matchIndex {
			matchIndex = index
			matchLength = len(term)
		}
	}
	if matchIndex == -1 {
		return trimRunes(normalized, 0, minInt(utf8.RuneCountInString(normalized), snippetContextRunes*2))
	}

	startRunes := utf8.RuneCountInString(normalized[:matchIndex])
	termRunes := utf8.RuneCountInString(normalized[matchIndex : matchIndex+matchLength])
	start := maxInt(0, startRunes-snippetContextRunes)
	end := minInt(utf8.RuneCountInString(normalized), startRunes+termRunes+snippetContextRunes)

	snippet := trimRunes(normalized, start, end)
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < utf8.RuneCountInString(normalized) {
		snippet += "…"
	}
	return snippet
}

func collapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func trimRunes(value string, start int, end int) string {
	runes := []rune(value)
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
