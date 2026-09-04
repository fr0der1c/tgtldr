package rollup

import (
	"strings"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/llmcontext"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPackSources(t *testing.T) {
	Convey("超大来源拆分后每个批次仍保留来源编号", t, func() {
		counter := llmcontext.NewCounter("custom-model")
		batches := packSources([]Source{{
			Header:  "[S001]\n群组：群 A\n每日摘要：\n",
			Content: strings.Repeat("内容", 900),
		}}, 500, counter)

		So(len(batches), ShouldBeGreaterThan, 1)
		for _, batch := range batches {
			So(batch, ShouldContainSubstring, "[S001]")
			So(counter.Count(batch), ShouldBeLessThanOrEqualTo, 500)
		}
	})
}
