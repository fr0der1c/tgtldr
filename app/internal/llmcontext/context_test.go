package llmcontext

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestResolveContextWindow(t *testing.T) {
	Convey("手动上下文长度应覆盖模型映射", t, func() {
		So(ResolveContextWindow("gpt-5.4", 65536), ShouldEqual, 65536)
	})

	Convey("已知模型应解析到各自的上下文窗口", t, func() {
		So(ResolveContextWindow("gpt-5.4", 0), ShouldEqual, 1050000)
		So(ResolveContextWindow("openai/gpt-5.4-mini", 0), ShouldEqual, 400000)
		So(ResolveContextWindow("gpt-5.2-2025-12-11", 0), ShouldEqual, 400000)
		So(ResolveContextWindow("gpt-4.1-mini", 0), ShouldEqual, 1047576)
		So(ResolveContextWindow("gpt-4o-2024-11-20", 0), ShouldEqual, 128000)
	})

	Convey("未知模型使用保守默认值", t, func() {
		So(ResolveContextWindow("custom-model", 0), ShouldEqual, DefaultContextWindow)
	})
}

func TestCounter(t *testing.T) {
	Convey("已知模型使用 tokenizer 计算中英文 token", t, func() {
		counter := NewCounter("gpt-5.4")
		So(counter.Count("hello world"), ShouldBeGreaterThan, 0)
		So(counter.Count("这是一个中文测试"), ShouldBeGreaterThan, 1)
	})

	Convey("未知模型的估算不会按四个中文字符计算一个 token", t, func() {
		counter := NewCounter("custom-model")
		So(counter.Count("这是一个中文测试"), ShouldEqual, 8)
	})
}

func TestPlanRequest(t *testing.T) {
	Convey("完整输入低于预算时应选择单次请求", t, func() {
		counter := NewCounter("gpt-5.4")
		plan := PlanRequest(counter, 32000, "system", "short input", 2000)

		So(plan.Fits, ShouldBeTrue)
		So(plan.InputBudget, ShouldBeGreaterThan, plan.InputTokens)
	})

	Convey("完整输入超过预算时应要求分块", t, func() {
		counter := NewCounter("custom-model")
		plan := PlanRequest(counter, 4096, "system", strings.Repeat("中", 4000), 1200)

		So(plan.Fits, ShouldBeFalse)
		So(plan.InputTokens, ShouldBeGreaterThan, plan.InputBudget)
	})
}

func TestPackTextParts(t *testing.T) {
	Convey("超大文本应切分且每一批都不超过预算", t, func() {
		counter := NewCounter("custom-model")
		batches := PackTextParts([]string{strings.Repeat("中", 1400)}, 500, counter, "\n---\n")

		So(len(batches), ShouldBeGreaterThan, 1)
		for _, batch := range batches {
			So(counter.Count(batch), ShouldBeLessThanOrEqualTo, 500)
		}
	})
}
