package store

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDailyDigestSourceCounts(t *testing.T) {
	Convey("来源统计区分有效摘要、无消息群组和遗漏群组", t, func() {
		participants, included, empty, omitted := dailyDigestSourceCounts([]model.DailyDigestSource{
			{Included: true},
			{OmissionReason: "no_messages"},
			{OmissionReason: "generation_failed"},
			{OmissionReason: "empty_content"},
		})

		So(participants, ShouldEqual, 4)
		So(included, ShouldEqual, 1)
		So(empty, ShouldEqual, 1)
		So(omitted, ShouldEqual, 2)
	})
}

func TestSortedDailyDigestSources(t *testing.T) {
	Convey("来源按标题和群组 ID 确定性排序且不修改输入", t, func() {
		sources := []model.DailyDigestSource{
			{ChatID: 3, ChatTitle: "群 B"},
			{ChatID: 2, ChatTitle: "群 A"},
			{ChatID: 1, ChatTitle: "群 A"},
		}
		result := sortedDailyDigestSources(sources)

		So([]int64{result[0].ChatID, result[1].ChatID, result[2].ChatID}, ShouldResemble, []int64{1, 2, 3})
		So([]int64{sources[0].ChatID, sources[1].ChatID, sources[2].ChatID}, ShouldResemble, []int64{3, 2, 1})
	})
}
