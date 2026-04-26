package config

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadOrCreateMasterKeyFile(t *testing.T) {
	Convey("未提供文件时应自动生成并持久化", t, func() {
		dir := t.TempDir()
		path := filepath.Join(dir, "master.key")

		first, err := loadOrCreateMasterKeyFile(path)
		So(err, ShouldBeNil)
		So(first, ShouldNotBeBlank)

		stat, err := os.Stat(path)
		So(err, ShouldBeNil)
		So(stat.Mode().Perm(), ShouldEqual, 0o600)

		second, err := loadOrCreateMasterKeyFile(path)
		So(err, ShouldBeNil)
		So(second, ShouldEqual, first)
	})
}
