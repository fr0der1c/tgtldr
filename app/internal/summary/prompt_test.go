package summary

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildStagePrompt(t *testing.T) {
	Convey("默认阶段提示词面向自由讨论群并保留额外要求", t, func() {
		prompt := buildStagePrompt("群里常说的 ATL 指的是 All Time Low。", "重点关注体验反馈。")

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
		prompt := buildFinalPrompt("", "")

		So(prompt, ShouldContainSubstring, "自由讨论群")
		So(prompt, ShouldContainSubstring, "## 今日主要结论")
		So(prompt, ShouldContainSubstring, "## 分话题总结")
		So(prompt, ShouldContainSubstring, "## 仍不确定的信息")
		So(strings.Contains(prompt, "## 待办事项"), ShouldBeFalse)
	})
}
