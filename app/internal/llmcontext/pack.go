package llmcontext

import "strings"

// PackTextParts 按 token 预算打包文本，单个超大文本会先切成可处理片段。
func PackTextParts(parts []string, tokenBudget int, counter Counter, separator string) []string {
	separatorTokens := counter.Count(separator)
	expanded := make([]string, 0, len(parts))
	for _, part := range parts {
		expanded = append(expanded, SplitTextToBudget(part, tokenBudget, counter)...)
	}

	batches := make([]string, 0, len(expanded))
	current := make([]string, 0)
	currentTokens := 0
	for _, part := range expanded {
		partTokens := counter.Count(part)
		additional := partTokens
		if len(current) > 0 {
			additional += separatorTokens
		}
		if len(current) > 0 && currentTokens+additional > tokenBudget {
			batches = append(batches, strings.Join(current, separator))
			current = current[:0]
			currentTokens = 0
			additional = partTokens
		}
		current = append(current, part)
		currentTokens += additional
	}
	if len(current) > 0 {
		batches = append(batches, strings.Join(current, separator))
	}
	return batches
}

// SplitTextToBudget 在段落或字符边界切分单个超大文本。
func SplitTextToBudget(text string, tokenBudget int, counter Counter) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if counter.Count(text) <= tokenBudget {
		return []string{text}
	}

	runes := []rune(text)
	parts := make([]string, 0)
	for len(runes) > 0 {
		low, high := 1, len(runes)
		for low < high {
			middle := (low + high + 1) / 2
			if counter.Count(string(runes[:middle])) <= tokenBudget {
				low = middle
				continue
			}
			high = middle - 1
		}
		cut := max(1, low)
		candidate := string(runes[:cut])
		if newline := strings.LastIndex(candidate, "\n"); newline > len(candidate)/2 {
			cut = max(1, len([]rune(candidate[:newline])))
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	return parts
}
