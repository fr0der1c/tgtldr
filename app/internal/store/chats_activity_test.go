package store

import (
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCompleteMessageActivity(t *testing.T) {
	Convey("群组消息活动应该补齐无消息日期", t, func() {
		location, err := time.LoadLocation("Asia/Shanghai")
		So(err, ShouldBeNil)

		start := time.Date(2026, 6, 1, 0, 0, 0, 0, location)
		got := completeMessageActivity(start, 4, []model.ChatMessageActivity{
			{Date: "2026-06-02", MessageCount: 3},
			{Date: "2026-06-04", MessageCount: 5},
		})

		So(got, ShouldResemble, []model.ChatMessageActivity{
			{Date: "2026-06-01", MessageCount: 0},
			{Date: "2026-06-02", MessageCount: 3},
			{Date: "2026-06-03", MessageCount: 0},
			{Date: "2026-06-04", MessageCount: 5},
		})
	})
}
