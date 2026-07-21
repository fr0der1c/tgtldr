package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMessageMediaAsset(t *testing.T) {
	Convey("图片会选择面积最大的尺寸", t, func() {
		message := &tg.Message{Media: &tg.MessageMediaPhoto{Photo: &tg.Photo{
			ID: 10, AccessHash: 20, FileReference: []byte("ref"),
			Sizes: []tg.PhotoSizeClass{
				&tg.PhotoSize{Type: "m", W: 320, H: 320},
				&tg.PhotoSize{Type: "x", W: 1280, H: 720},
			},
		}}}

		asset, ok := messageMediaAsset(7, message)

		So(ok, ShouldBeTrue)
		So(asset.Kind, ShouldEqual, "photo")
		So(asset.ThumbSize, ShouldEqual, "x")
		So(asset.TelegramAccountID, ShouldEqual, int64(7))
	})

	Convey("普通视频和文件会保留文件名及大小", t, func() {
		message := &tg.Message{Media: &tg.MessageMediaDocument{
			Video: true,
			Document: &tg.Document{
				ID: 11, AccessHash: 21, FileReference: []byte("ref"), Size: 2048,
				MimeType:   "video/mp4",
				Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: "demo.mp4"}},
			},
		}}

		asset, ok := messageMediaAsset(8, message)

		So(ok, ShouldBeTrue)
		So(asset.Kind, ShouldEqual, "video")
		So(asset.FileName, ShouldEqual, "demo.mp4")
		So(asset.FileSize, ShouldEqual, int64(2048))
	})

	Convey("三种 Telegram 贴纸都会进入下载队列并保留格式", t, func() {
		cases := []struct {
			mimeType  string
			extension string
		}{
			{mimeType: "image/webp", extension: ".webp"},
			{mimeType: "application/x-tgsticker", extension: ".tgs"},
			{mimeType: "video/webm", extension: ".webm"},
		}
		for index, item := range cases {
			message := &tg.Message{Media: &tg.MessageMediaDocument{Document: &tg.Document{
				ID: int64(12 + index), MimeType: item.mimeType,
				Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeSticker{}},
			}}}

			asset, ok := messageMediaAsset(9, message)

			So(ok, ShouldBeTrue)
			So(asset.Kind, ShouldEqual, "sticker")
			So(asset.FileName, ShouldEndWith, item.extension)
		}
	})
}

func TestSanitizeFileName(t *testing.T) {
	Convey("文件名不能跳出资源目录或携带控制字符", t, func() {
		So(sanitizeFileName("../../report\n.pdf"), ShouldEqual, "report_.pdf")
		So(sanitizeFileName(""), ShouldEqual, "file")
	})
}

func TestStoredMediaAsset(t *testing.T) {
	Convey("升级前保存的图片 JSON 可以直接恢复下载引用", t, func() {
		raw := `{"Media":{"Photo":{"ID":99,"AccessHash":88,"FileReference":"cmVm","Sizes":[{"Type":"m","W":320,"H":200},{"Type":"x","W":800,"H":600}]}}}`

		asset, ok, err := storedMediaAsset(3, raw)

		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
		So(asset.TelegramFileID, ShouldEqual, int64(99))
		So(asset.ThumbSize, ShouldEqual, "x")
		So(string(asset.FileReference), ShouldEqual, "ref")
	})

	Convey("升级前保存的贴纸 JSON 会自动补建下载记录", t, func() {
		raw := `{"Media":{"Document":{"ID":100,"MimeType":"image/webp","Attributes":[{"Stickerset":{"ID":1}},{"FileName":"sticker.webp"}]}}}`

		asset, ok, err := storedMediaAsset(3, raw)

		So(err, ShouldBeNil)
		So(ok, ShouldBeTrue)
		So(asset.Kind, ShouldEqual, "sticker")
	})
}
