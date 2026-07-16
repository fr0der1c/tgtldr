package api

import (
	"path/filepath"
	"testing"

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
		So(contentDisposition("document", "report.pdf"), ShouldStartWith, "attachment;")
		So(contentDisposition("document", "bad\nname.pdf"), ShouldNotContainSubstring, "\n")
	})
}
