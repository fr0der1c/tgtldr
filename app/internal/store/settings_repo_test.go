package store

import (
	"testing"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNormalizeAppSettingsLanguage(t *testing.T) {
	Convey("语言设置为空或非法时默认使用中文", t, func() {
		So(normalizeAppSettings(model.AppSettings{}).Language, ShouldEqual, model.LanguageZhCN)
		So(normalizeAppSettings(model.AppSettings{Language: "fr"}).Language, ShouldEqual, model.LanguageZhCN)
	})

	Convey("语言设置为英文时保留英文", t, func() {
		settings := normalizeAppSettings(model.AppSettings{Language: model.LanguageEN})

		So(settings.Language, ShouldEqual, model.LanguageEN)
	})
}
