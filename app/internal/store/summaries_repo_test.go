package store

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildSummaryWhereClause(t *testing.T) {
	Convey("multi-term search should build an AND where clause", t, func() {
		params := SummaryListParams{
			Query:    "alpha beta",
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
		So(whereClause, ShouldContainSubstring, "s.summary_type = 'daily' and c.delivery_mode = 'bot'")
		So(whereClause, ShouldContainSubstring, "s.summary_type = 'rolling' and c.rolling_summary_bot_enabled = true")
		So(whereClause, ShouldContainSubstring, "s.delivered_at is null and s.delivery_error = ''")
		So(strings.Count(whereClause, "s.content ilike"), ShouldEqual, 2)
		So(len(args), ShouldEqual, 6)
		So(args[4], ShouldEqual, "%alpha%")
		So(args[5], ShouldEqual, "%beta%")
	})
}

func TestSummarizeSearchMatch(t *testing.T) {
	Convey("content match should return a clean content snippet", t, func() {
		snippet, fields := summarizeSearchMatch(
			"## Today\n- **alpha** has repeated discussion, while beta is also mentioned.",
			"example group",
			[]string{"alpha", "beta"},
		)

		So(snippet, ShouldContainSubstring, "alpha")
		So(snippet, ShouldContainSubstring, "beta")
		So(snippet, ShouldNotContainSubstring, "##")
		So(snippet, ShouldNotContainSubstring, "**")
		So(fields, ShouldResemble, []string{"content"})
	})

	Convey("title-only match should return the title-match hint", t, func() {
		snippet, fields := summarizeSearchMatch(
			"today discussed routing and meals",
			"alpha discussion group",
			[]string{"alpha"},
		)

		So(snippet, ShouldEqual, "匹配到群组名称")
		So(fields, ShouldResemble, []string{"title"})
	})
}
