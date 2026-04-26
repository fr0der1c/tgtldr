package store

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildSummaryWhereClause(t *testing.T) {
	Convey("多关键词搜索应按 AND 语义构造 where 条件", t, func() {
		params := SummaryListParams{
			Query:    "良心云 联通",
			Status:   "succeeded",
			Delivery: "pending",
			DateFrom: "2026-04-01",
			DateTo:   "2026-04-18",
			ChatID:   7,
		}

		whereClause, args := buildSummaryWhereClause(normalizeSummaryListParams(params), searchTerms(params.Query))

		So(whereClause, ShouldContainSubstring, "s.chat_id = $1")
		So(whereClause, ShouldContainSubstring, "s.status = $2")
		So(whereClause, ShouldContainSubstring, "s.summary_date >= $3::date")
		So(whereClause, ShouldContainSubstring, "s.summary_date <= $4::date")
		So(whereClause, ShouldContainSubstring, "c.delivery_mode = 'bot' and s.delivered_at is null and s.delivery_error = ''")
		So(strings.Count(whereClause, "s.content ilike"), ShouldEqual, 2)
		So(len(args), ShouldEqual, 6)
		So(args[4], ShouldEqual, "%良心云%")
		So(args[5], ShouldEqual, "%联通%")
	})
}

func TestSummarizeSearchMatch(t *testing.T) {
	Convey("正文命中时应该返回摘要片段和 content 字段", t, func() {
		snippet, fields := summarizeSearchMatch(
			"## 今日主要结论\n- **良心云** 最近的稳定性被反复讨论，很多人认为联通线路波动明显，但备用价值还在。",
			"翻翻墙讨论群",
			[]string{"良心云", "联通"},
		)

		So(snippet, ShouldContainSubstring, "良心云")
		So(snippet, ShouldContainSubstring, "联通")
		So(snippet, ShouldNotContainSubstring, "##")
		So(snippet, ShouldNotContainSubstring, "**")
		So(fields, ShouldResemble, []string{"content"})
	})

	Convey("只命中群名时应该返回群名命中的提示", t, func() {
		snippet, fields := summarizeSearchMatch(
			"今天主要讨论的是线路和套餐。",
			"良心云交流群",
			[]string{"良心云"},
		)

		So(snippet, ShouldEqual, "匹配到群组名称")
		So(fields, ShouldResemble, []string{"title"})
	})
}
