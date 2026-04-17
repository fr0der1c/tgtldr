package summary

import (
	"testing"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestResolveSummaryBudget(t *testing.T) {
	Convey("自动模式不会显式传输出上限，并会使用默认并行度和动态预算", t, func() {
		budget := resolveSummaryBudget(model.AppSettings{
			OpenAIOutputMode:   model.OutputModeAuto,
			SummaryParallelism: 0,
		}, "gpt-5.4", "system prompt")

		So(budget.StageRequestMax, ShouldEqual, 0)
		So(budget.FinalRequestMax, ShouldEqual, 0)
		So(budget.Parallelism, ShouldEqual, 2)
		So(budget.ChunkTokenBudget, ShouldBeGreaterThan, 50000)
	})

	Convey("手动模式会沿用自定义输出上限，并限制阶段摘要输出", t, func() {
		budget := resolveSummaryBudget(model.AppSettings{
			OpenAIOutputMode:     model.OutputModeManual,
			OpenAIMaxOutputToken: 2600,
			SummaryParallelism:   3,
		}, "gpt-4.1", "system prompt")

		So(budget.StageRequestMax, ShouldEqual, defaultStageOutputReserve)
		So(budget.FinalRequestMax, ShouldEqual, 2600)
		So(budget.Parallelism, ShouldEqual, 3)
	})
}
