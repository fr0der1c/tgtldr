package config

import (
	"os"
	"path/filepath"
	"runtime"
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
		if runtime.GOOS != "windows" {
			So(stat.Mode().Perm(), ShouldEqual, 0o600)
		}

		second, err := loadOrCreateMasterKeyFile(path)
		So(err, ShouldBeNil)
		So(second, ShouldEqual, first)
	})
}

func TestLoadReadsReadAPIToken(t *testing.T) {
	t.Setenv("TGTLDR_MASTER_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("TGTLDR_READ_API_TOKEN", "read-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ReadAPIToken != "read-token" {
		t.Fatalf("read API token = %q", cfg.ReadAPIToken)
	}
}
