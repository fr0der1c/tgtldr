package summary

import (
	"strings"
	"testing"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildStagePrompt(t *testing.T) {
	Convey("默认阶段提示词面向自由讨论群并保留额外要求", t, func() {
		prompt := buildStagePrompt(model.LanguageZhCN, "群里常说的 ATL 指的是 All Time Low。", "重点关注体验反馈。")

		So(prompt, ShouldContainSubstring, "自由发散讨论")
		So(prompt, ShouldContainSubstring, "群聊背景：\n群里常说的 ATL 指的是 All Time Low。")
		So(prompt, ShouldContainSubstring, "reply_to 和 reply_excerpt")
		So(prompt, ShouldContainSubstring, "## 分话题讨论摘要")
		So(prompt, ShouldContainSubstring, "额外要求：\n重点关注体验反馈。")
		So(strings.Contains(prompt, "待办事项"), ShouldBeFalse)
	})
}

func TestBuildFinalPrompt(t *testing.T) {
	Convey("默认最终提示词聚焦话题与群体判断", t, func() {
		prompt := buildFinalPrompt(model.LanguageZhCN, "", "")

		So(prompt, ShouldContainSubstring, "自由讨论群")
		So(prompt, ShouldContainSubstring, "## 今日主要结论")
		So(prompt, ShouldContainSubstring, "## 分话题总结")
		So(prompt, ShouldContainSubstring, "## 仍不确定的信息")
		So(strings.Contains(prompt, "## 待办事项"), ShouldBeFalse)
	})

	Convey("英文最终提示词要求英文输出", t, func() {
		prompt := buildFinalPrompt(model.LanguageEN, "ATL means All Time Low.", "Keep important links.")

		So(prompt, ShouldContainSubstring, "Write in English")
		So(prompt, ShouldContainSubstring, "## Key Takeaways")
		So(prompt, ShouldContainSubstring, "Group context:\nATL means All Time Low.")
		So(prompt, ShouldContainSubstring, "Additional requirements:\nKeep important links.")
		So(strings.Contains(prompt, "## 今日主要结论"), ShouldBeFalse)
	})
}
