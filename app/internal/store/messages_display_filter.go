package store

import (
	"fmt"
	"strings"
)

type MessageDisplayFilter struct {
	Senders  []string
	Keywords []string
}

func appendMessageDisplayFilter(
	alias string,
	args []any,
	filter MessageDisplayFilter,
) (string, []any) {
	senders := normalizeMessageFilterValues(filter.Senders, "")
	keywords := normalizeMessageKeywordValues(filter.Keywords)
	clauses := make([]string, 0, 2)
	if len(senders) > 0 {
		args = append(args, senders)
		index := len(args)
		clauses = append(clauses, fmt.Sprintf(`not (
			lower(btrim(%[1]s.sender_name)) = any($%[2]d::text[])
			or lower(btrim(%[1]s.sender_username)) = any($%[2]d::text[])
			or ('@' || lower(btrim(%[1]s.sender_username))) = any($%[2]d::text[])
		)`, alias, index))
	}
	if len(keywords) > 0 {
		args = append(args, keywords)
		clauses = append(clauses, fmt.Sprintf(
			"not (lower(coalesce(nullif(%s.text_content, ''), %s.caption, '')) like any($%d::text[]))",
			alias, alias, len(args),
		))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " and " + strings.Join(clauses, " and "), args
}

func normalizeMessageKeywordValues(values []string) []string {
	normalized := normalizeMessageFilterValues(values, "")
	for index, value := range normalized {
		value = strings.ReplaceAll(value, `\`, `\\`)
		value = strings.ReplaceAll(value, `%`, `\%`)
		value = strings.ReplaceAll(value, `_`, `\_`)
		normalized[index] = "%" + value + "%"
	}
	return normalized
}

func normalizeMessageFilterValues(values []string, wrapper string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		normalized = append(normalized, wrapper+value+wrapper)
	}
	return normalized
}
