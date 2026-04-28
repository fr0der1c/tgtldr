package bot

import (
	"html"
	"testing"

	"github.com/frederic/tgtldr/app/internal/model"
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

	Convey("超长消息会自动截断并追加提示", t, func() {
		body := stringsJoin(
			"## 今日主要结论",
			"",
			"- **很长的摘要** "+repeatText("机场稳定性讨论。", 500),
			"",
			"## 分话题总结",
			"",
			"- "+repeatText("第二段内容。", 500),
		)

		output := formatTelegramMessage(body, model.LanguageZhCN)

		So(telegramVisibleLength(output) <= telegramMessageVisibleLimit, ShouldBeTrue)
		So(output, ShouldContainSubstring, htmlEscape(telegramTruncationNotice(model.LanguageZhCN)))
		So(output, ShouldContainSubstring, "<b>【今日主要结论】</b>")
		So(output, ShouldContainSubstring, "</b>")
	})

	Convey("短消息不会追加截断提示", t, func() {
		output := formatTelegramMessage("## 今日主要结论\n\n- 一切正常", model.LanguageZhCN)

		So(output, ShouldNotContainSubstring, htmlEscape(telegramTruncationNotice(model.LanguageZhCN)))
		So(output, ShouldContainSubstring, "<b>【今日主要结论】</b>")
	})

	Convey("英文语言下截断提示使用英文", t, func() {
		body := "## Key Takeaways\n\n- " + repeatText("Long summary content. ", 500)

		output := formatTelegramMessage(body, model.LanguageEN)

		So(output, ShouldContainSubstring, htmlEscape(telegramTruncationNotice(model.LanguageEN)))
		So(output, ShouldNotContainSubstring, htmlEscape(telegramTruncationNotice(model.LanguageZhCN)))
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

func repeatText(text string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += text
	}
	return result
}

func htmlEscape(input string) string {
	return html.EscapeString(input)
}
