package bot

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFormatTelegramHTML(t *testing.T) {
	Convey("格式化 Markdown 为 Telegram HTML", t, func() {
		input := stringsJoin(
			"## 今日主要结论",
			"",
			"- **节点 A** 表现稳定",
			"- 查看 [文档](https://example.com)",
			"",
			"```",
			"line 1",
			"line 2",
			"```",
			"",
			"`inline` 代码",
		)

		output := formatTelegramHTML(input)

		So(output, ShouldContainSubstring, "<b>【今日主要结论】</b>")
		So(output, ShouldContainSubstring, "• <b>节点 A</b> 表现稳定")
		So(output, ShouldContainSubstring, `<a href="https://example.com">文档</a>`)
		So(output, ShouldContainSubstring, "<pre>line 1\nline 2</pre>")
		So(output, ShouldContainSubstring, "<code>inline</code> 代码")
	})

	Convey("三级标题保持简洁粗体", t, func() {
		output := formatTelegramHTML("### 分话题总结")
		So(output, ShouldEqual, "<b>分话题总结</b>")
	})
}

func stringsJoin(lines ...string) string {
	result := ""
	for index, line := range lines {
		if index > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
