package api

import (
	"bytes"
	"compress/gzip"
	"path/filepath"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSafeMediaPath(t *testing.T) {
	Convey("资源路径只能位于媒体根目录内", t, func() {
		root := t.TempDir()
		path, err := safeMediaPath(root, "messages/1/photo.jpg")
		So(err, ShouldBeNil)
		So(path, ShouldEqual, filepath.Join(root, "messages", "1", "photo.jpg"))

		_, err = safeMediaPath(root, "../../secret")
		So(err, ShouldNotBeNil)
	})
}

func TestContentDisposition(t *testing.T) {
	Convey("可预览媒体使用 inline，普通文件强制下载", t, func() {
		So(contentDisposition("photo", "photo.jpg"), ShouldStartWith, "inline;")
		So(contentDisposition("sticker", "sticker.tgs"), ShouldStartWith, "inline;")
		So(contentDisposition("document", "report.pdf"), ShouldStartWith, "attachment;")
		So(contentDisposition("document", "bad\nname.pdf"), ShouldNotContainSubstring, "\n")
	})
}

func TestReadTGSJSON(t *testing.T) {
	Convey("TGS 文件会解压为 Lottie JSON", t, func() {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, err := writer.Write([]byte(`{"v":"5.7.4"}`))
		So(err, ShouldBeNil)
		So(writer.Close(), ShouldBeNil)

		content, err := readTGSJSON(&compressed)

		So(err, ShouldBeNil)
		So(string(content), ShouldEqual, `{"v":"5.7.4"}`)
	})
}

func TestManualMediaResponse(t *testing.T) {
	Convey("策略暂停的附件允许用户手动下载", t, func() {
		response := mediaResponse(model.MediaAsset{ID: 12, Status: "manual"})

		So(response.Status, ShouldEqual, "manual")
		So(response.CanDownload, ShouldBeTrue)
		So(response.CanRetry, ShouldBeFalse)
		So(response.ContentURL, ShouldBeEmpty)
	})
}
