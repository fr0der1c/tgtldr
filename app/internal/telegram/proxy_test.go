package telegram

import (
	"net/url"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParseTelegramProxyURL(t *testing.T) {
	Convey("支持 socks5h 代理地址", t, func() {
		proxyURL, err := parseTelegramProxyURL(" socks5h://user:pass@127.0.0.1:7890 ")

		So(err, ShouldBeNil)
		So(proxyURL.Scheme, ShouldEqual, "socks5h")
		So(proxyURL.Host, ShouldEqual, "127.0.0.1:7890")
		So(proxyURL.User.String(), ShouldEqual, url.UserPassword("user", "pass").String())
	})

	Convey("支持 socks5 代理地址", t, func() {
		proxyURL, err := parseTelegramProxyURL("socks5://proxy.example.com:1080")

		So(err, ShouldBeNil)
		So(proxyURL.Scheme, ShouldEqual, "socks5")
		So(proxyURL.Host, ShouldEqual, "proxy.example.com:1080")
	})

	Convey("拒绝不支持的协议", t, func() {
		_, err := parseTelegramProxyURL("http://127.0.0.1:7890")

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "unsupported proxy scheme")
	})

	Convey("代理地址必须包含 host", t, func() {
		_, err := parseTelegramProxyURL("socks5h://")

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "must include host")
	})
}

func TestTelegramProxyResolver(t *testing.T) {
	Convey("合法代理地址会创建 gotd resolver", t, func() {
		resolver, err := telegramProxyResolver("socks5h://127.0.0.1:7890")

		So(err, ShouldBeNil)
		So(resolver, ShouldNotBeNil)
	})
}
