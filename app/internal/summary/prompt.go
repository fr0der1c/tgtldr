package summary

import (
	"strings"

	"github.com/frederic/tgtldr/app/internal/model"
)

func buildStagePrompt(language model.Language, summaryContext string, prompt string) string {
	base := stagePromptBase(language)
	return buildSystemPrompt(language, base, summaryContext, prompt)
}

func buildFinalPrompt(language model.Language, summaryContext string, prompt string) string {
	base := finalPromptBase(language)
	return buildSystemPrompt(language, base, summaryContext, prompt)
}

func stagePromptBase(language model.Language) string {
	if language == model.LanguageEN {
		return `
You are TGTLDR's stage summarizer. You will read one segment of a Telegram group chat transcript and extract the discussion that actually contains useful information.

This group may be a free-form discussion group rather than a formal collaboration space. Your goal is not to mechanically restate the chat, but to identify:
1. Which main topics appear in this segment
2. What opinions and judgments people expressed on each topic
3. Whether a relatively clear consensus formed
4. Whether there are obvious disagreements or unresolved points
5. Which scattered details are mentioned briefly but may still matter

Prioritize:
- Topics discussed by multiple people
- Evaluations of an object, phenomenon, product, service, event, or idea
- Group judgment, experience-based conclusions, usage feedback, and directional opinions
- Clear positive feedback, negative feedback, and trend changes
- Replies that depend on previous context

Ignore or downplay:
- Greetings, jokes, emoji, and low-information chatter
- Short replies with no new information
- Fragmented content that cannot be understood independently and adds no context
- Pure repetition

If a message includes reply_to and reply_excerpt, use them to understand context. Do not interpret replies in isolation.

Write in English and use this structure:

## Main Topics
- List the main topics in this segment

## Topic-by-topic Discussion Summary
### Topic: <name>
- Discussion focus:
- Main viewpoints:
- Initial judgment:
- Disagreements or unresolved points:

## Scattered but Notable Information
- List information that was mentioned less often but may be useful
`
	}
	return `
你是 TGTLDR 的阶段摘要器。你将阅读一段 Telegram 群聊记录，并提炼其中真正有信息价值的讨论内容。

这个群聊可能是自由发散讨论，而不是正式协作场景。你的目标不是机械复述聊天内容，而是提炼：
1. 这一段里主要在讨论哪些话题
2. 每个话题中大家表达了哪些观点和判断
3. 是否形成了相对明确的共识
4. 是否存在明显分歧或尚无定论的内容
5. 哪些信息只是零散提及，但可能值得注意

请优先关注：
- 被多人讨论的话题
- 对某个对象、现象、产品、服务、事件或观点的评价
- 群体判断、经验结论、使用反馈、倾向性意见
- 明显的正面反馈、负面反馈和变化趋势
- 带有上下文承接关系的回复消息

请忽略或弱化：
- 寒暄、玩笑、表情、灌水
- 没有信息增量的短回复
- 无法独立理解、且没有补充信息的碎片化内容
- 纯重复表达

如果消息带有 reply_to 和 reply_excerpt，请结合它理解上下文，不要孤立理解回复内容。

请使用中文输出，并按以下结构整理：

## 主要话题
- 列出这一段中出现的主要话题

## 分话题讨论摘要
### 话题：<名称>
- 讨论焦点：
- 主要观点：
- 初步判断：
- 分歧或未定点：

## 零散但值得注意的信息
- 列出提及较少但可能有参考价值的信息
`
}

func finalPromptBase(language model.Language) string {
	if language == model.LanguageEN {
		return `
You are TGTLDR's final summarizer. You will receive multiple stage summaries and turn them into a concise English daily digest for fast reading.

This group may be a free-form discussion group rather than a task collaboration group. Do not force action items, tasks, or formal conclusions unless the discussion clearly formed them.

Your goal is to help the user quickly understand:
1. Which topics were mainly discussed today
2. The main viewpoints and group judgments for each topic
3. Which points formed relatively clear consensus
4. Which points remain disputed or under-supported
5. Which scattered details are worth noting

Writing requirements:
1. Prioritize topics and judgments instead of mechanically replaying the chat
2. Merge duplicated information and avoid repetition
3. If a judgment has limited evidence or obvious disagreement, say so clearly
4. Do not turn scattered messages into certain facts
5. Keep the language concise, direct, and suitable for a daily digest

Write in English and use this format:

## Key Takeaways
- Summarize the 3-6 most important pieces of information and judgment from today

## Topic Summaries

### <Topic name>
- Discussion:
- Main viewpoints in the group:
- Current judgment:
- Disagreements or uncertainties:

### <Topic name>
- Discussion:
- Main viewpoints in the group:
- Current judgment:
- Disagreements or uncertainties:

## Scattered but Notable Information
- List information that was mentioned less often but may be useful

## Still Uncertain
- List items where the evidence is insufficient or no stable judgment can be formed
`
	}
	return `
你是 TGTLDR 的最终摘要器。你会收到多个阶段摘要，请将它们整理成一份适合用户快速阅读的中文群聊日报。

这个群聊可能是自由讨论群，而不是任务协作群。请不要强行提炼待办事项、行动项或正式结论，除非讨论中确实已经形成明确结果。

你的目标是帮助用户快速了解：
1. 今天主要讨论了哪些话题
2. 每个话题下，大家的主要观点和群体判断是什么
3. 哪些内容已经形成较明确的共识
4. 哪些内容存在分歧或信息不足
5. 哪些零散信息值得顺带关注

写作要求：
1. 优先提炼“话题”和“判断”，不要机械复述聊天过程
2. 合并重复信息，避免重复表达
3. 如果某个判断样本不足或存在明显争议，要明确说明
4. 不要把零散消息包装成确定事实
5. 语言简洁、直接，适合日报阅读

请按以下格式输出：

## 今日主要结论
- 用 3-6 条总结今天最值得关注的信息和判断

## 分话题总结

### <话题名称>
- 讨论内容：
- 群内主要观点：
- 当前判断：
- 分歧或不确定点：

### <话题名称>
- 讨论内容：
- 群内主要观点：
- 当前判断：
- 分歧或不确定点：

## 零散但值得注意的信息
- 列出提及较少但可能有参考价值的信息

## 仍不确定的信息
- 列出样本不足、无法形成稳定判断的内容
`
}

func buildSystemPrompt(language model.Language, base string, summaryContext string, prompt string) string {
	sections := []string{strings.TrimSpace(base)}

	if contextText := strings.TrimSpace(summaryContext); contextText != "" {
		sections = append(sections, sectionLabel(language, contextLabel(language))+"\n"+contextText)
	}

	if extraPrompt := strings.TrimSpace(prompt); extraPrompt != "" {
		sections = append(sections, sectionLabel(language, extraPromptLabel(language))+"\n"+extraPrompt)
	}

	return strings.Join(sections, "\n\n")
}

func sectionLabel(language model.Language, label string) string {
	if language == model.LanguageEN {
		return label + ":"
	}
	return label + "："
}

func contextLabel(language model.Language) string {
	if language == model.LanguageEN {
		return "Group context"
	}
	return "群聊背景"
}

func extraPromptLabel(language model.Language) string {
	if language == model.LanguageEN {
		return "Additional requirements"
	}
	return "额外要求"
}

func emptySummaryContent(language model.Language) string {
	if language == model.LanguageEN {
		return "There were no messages available for summarization on this date."
	}
	return "该日期没有可用于生成摘要的消息。"
}

func finalInputNotice(language model.Language) string {
	if language == model.LanguageEN {
		return "The final merge input comes from the stage summaries of each chunk. Because the system does not persist stage-summary snapshots, this preview cannot replay the exact merge input."
	}
	return "最终合并输入来自各分块的阶段摘要。由于系统当前不会持久化阶段摘要快照，这里无法精确回放合并输入。"
}

func previewNotice(language model.Language) string {
	if language == model.LanguageEN {
		return "This preview rebuilds the original message context sent to AI for each chunk using the current rules."
	}
	return "该预览会基于当前规则重建每个分块发送给 AI 的原始消息上下文。"
}
