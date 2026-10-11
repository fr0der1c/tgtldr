package dailydigest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

var deliverySourceRun = regexp.MustCompile(`(?:[ \t]*\[S[0-9]{3,}\])+`)

// deliveryContentWithSources 沿用模型输入的编号顺序，将相邻引用合并为去重的群名，未知编号保留原文。
func deliveryContentWithSources(language model.Language, item model.DailyDigest) string {
	titles := make(map[string]string)
	number := 0
	for _, source := range item.Sources {
		if !source.Included {
			continue
		}
		number++
		titles[fmt.Sprintf("[S%03d]", number)] = strings.TrimSpace(source.ChatTitle)
	}
	return deliverySourceRun.ReplaceAllStringFunc(item.Content, func(run string) string {
		names := make([]string, 0)
		unknown := make([]string, 0)
		seen := make(map[string]bool)
		for _, marker := range deliverySourceMarker.FindAllString(run, -1) {
			marker = strings.TrimSpace(marker)
			title := titles[marker]
			if title == "" {
				unknown = append(unknown, marker)
				continue
			}
			if !seen[title] {
				names = append(names, title)
				seen[title] = true
			}
		}
		if len(names) == 0 {
			return run
		}
		label := "（来源：" + strings.Join(names, "、") + "）"
		if language == model.LanguageEN {
			label = " (Sources: " + strings.Join(names, ", ") + ")"
		}
		if len(unknown) > 0 {
			label += " " + strings.Join(unknown, " ")
		}
		return label
	})
}
