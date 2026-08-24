package summary

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
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
		So(budget.ContextWindow, ShouldEqual, 1050000)
		So(budget.ChunkTokenBudget, ShouldBeGreaterThan, 900000)
		So(budget.FinalReserve, ShouldEqual, 4096)
	})

	Convey("手动模式会沿用自定义输出上限，并限制阶段摘要输出", t, func() {
		budget := resolveSummaryBudget(model.AppSettings{
			OpenAIOutputMode:     model.OutputModeManual,
			OpenAIMaxOutputToken: 2600,
			SummaryParallelism:   3,
		}, "gpt-4.1", "system prompt")

		So(budget.StageRequestMax, ShouldEqual, defaultStageOutputReserve)
		So(budget.FinalRequestMax, ShouldEqual, 2600)
		So(budget.FinalReserve, ShouldEqual, 2600)
		So(budget.Parallelism, ShouldEqual, 3)
	})

	Convey("手动上下文长度覆盖模型自动识别", t, func() {
		budget := resolveSummaryBudget(model.AppSettings{
			OpenAIContextWindowMode:   model.ContextWindowModeManual,
			OpenAIContextWindowTokens: 64000,
		}, "gpt-5.4", "system prompt")

		So(budget.ContextWindow, ShouldEqual, 64000)
		So(budget.ChunkTokenBudget, ShouldBeLessThan, 64000)
	})
}
