package telegram

import (
	"errors"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/gotd/td/tgerr"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoggedOutAuth(t *testing.T) {
	Convey("Telegram 实际未授权时会清除旧 session 并标记为未登录", t, func() {
		current := model.TelegramAuth{
			PhoneNumber:     "+8612345678901",
			TelegramUserID:  123,
			TelegramName:    "Frederik",
			TelegramHandle:  "frederic",
			Status:          "authorized",
			SessionData:     []byte("stale-session"),
			LastConnectedAt: time.Date(2026, 4, 26, 14, 9, 20, 0, time.UTC),
		}

		next := loggedOutAuth(current)

		So(next.Status, ShouldEqual, "logged_out")
		So(next.SessionData, ShouldBeNil)
		So(next.PhoneNumber, ShouldEqual, current.PhoneNumber)
		So(next.TelegramUserID, ShouldEqual, current.TelegramUserID)
		So(next.LastConnectedAt, ShouldEqual, current.LastConnectedAt)
	})
}

func TestInvalidTelegramSessionError(t *testing.T) {
	Convey("Telegram 永久认证错误会被识别为失效 session", t, func() {
		So(isInvalidTelegramSessionError(tgerr.New(406, "AUTH_KEY_DUPLICATED")), ShouldBeTrue)
		So(isInvalidTelegramSessionError(tgerr.New(401, "AUTH_KEY_UNREGISTERED")), ShouldBeTrue)
		So(isInvalidTelegramSessionError(tgerr.New(400, "SESSION_EXPIRED")), ShouldBeTrue)
	})

	Convey("普通错误不会被识别为失效 session", t, func() {
		So(isInvalidTelegramSessionError(errors.New("temporary network error")), ShouldBeFalse)
		So(isInvalidTelegramSessionError(nil), ShouldBeFalse)
	})
}
