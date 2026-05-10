package store

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
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

	Convey("重试退避参数为空时使用默认值", t, func() {
		settings := normalizeAppSettings(model.AppSettings{})

		So(settings.SummaryRetryBackoffBaseMinutes, ShouldEqual, model.DefaultSummaryRetryBackoffBaseMinutes)
		So(settings.SummaryRetryBackoffMultiplier, ShouldEqual, model.DefaultSummaryRetryBackoffMultiplier)
	})

	Convey("OpenAI 调用方式为空或非法时默认使用流式", t, func() {
		So(normalizeAppSettings(model.AppSettings{}).OpenAIRequestMode, ShouldEqual, model.OpenAIRequestModeStream)
		So(normalizeAppSettings(model.AppSettings{OpenAIRequestMode: "invalid"}).OpenAIRequestMode, ShouldEqual, model.OpenAIRequestModeStream)
	})

	Convey("OpenAI 调用方式为非流式时保留配置", t, func() {
		settings := normalizeAppSettings(model.AppSettings{OpenAIRequestMode: model.OpenAIRequestModeNonStream})

		So(settings.OpenAIRequestMode, ShouldEqual, model.OpenAIRequestModeNonStream)
	})
}
