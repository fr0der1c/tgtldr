package dailydigest

import (
	"strings"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// TestDeliverySourceNames 验证单群、多群、重复标题、未知引用和中英文投递的来源对应关系。
func TestDeliverySourceNames(t *testing.T) {
	item := model.DailyDigest{Sources: []model.DailyDigestSource{
		{ChatTitle: "Duang VPS交流群", Included: true},
		{ChatTitle: "省略群"},
		{ChatTitle: "翻翻墙 | MiaoKo", Included: true},
		{ChatTitle: "Duang VPS交流群", Included: true},
	}}
	for _, test := range []struct {
		name, input, want string
		language          model.Language
	}{
		{"single", "发现[S001]", "发现（来源：Duang VPS交流群）", model.LanguageZhCN},
		{"multiple", "发现 [S002][S001] [S003] [S002]", "发现（来源：翻翻墙 | MiaoKo、Duang VPS交流群）", model.LanguageZhCN},
		{"separate", "发现[S001]\n\n另一条 [S002]", "发现（来源：Duang VPS交流群）\n\n另一条（来源：翻翻墙 | MiaoKo）", model.LanguageZhCN},
		{"unknown", "发现 [S009]", "发现 [S009]", model.LanguageZhCN},
		{"no citation", "未含引用的历史正文", "未含引用的历史正文", model.LanguageZhCN},
		{"English", "Finding [S001] [S002]", "Finding (Sources: Duang VPS交流群, 翻翻墙 | MiaoKo)", model.LanguageEN},
	} {
		t.Run(test.name, func(t *testing.T) {
			item.Content = test.input
			if got := deliveryContentWithSources(test.language, item); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			if item.Content != test.input {
				t.Fatal("stored content was modified")
			}
		})
	}
	item.Sources = nil
	item.Content = "发现 [S001]"
	if got := deliveryContentWithSources(model.LanguageZhCN, item); !strings.Contains(got, "[S001]") {
		t.Fatal("unavailable source metadata must not create a guessed title")
	}
}
