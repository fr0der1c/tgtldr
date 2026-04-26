package localauth

import (
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidatePassword(t *testing.T) {
	Convey("访问密码长度需要落在允许范围内", t, func() {
		So(validatePassword("1234567"), ShouldEqual, ErrPasswordTooShort)
		So(validatePassword(strings.Repeat("a", 129)), ShouldEqual, ErrPasswordTooLong)
		So(validatePassword("admin123"), ShouldBeNil)
	})
}

func TestLoginLimiter(t *testing.T) {
	Convey("连续登录失败后会短暂限速", t, func() {
		limiter := &loginLimiter{}
		now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

		for i := 0; i < maxFailedLoginAttempts; i++ {
			So(limiter.beforeLogin(now), ShouldBeNil)
			limiter.recordFailure(now)
		}

		err := limiter.beforeLogin(now)
		So(err, ShouldHaveSameTypeAs, &LoginRateLimitError{})
		So(err.(*LoginRateLimitError).RetryAfterSeconds(), ShouldEqual, 60)

		So(limiter.beforeLogin(now.Add(loginLockoutDuration+time.Second)), ShouldBeNil)
		limiter.recordSuccess()
		So(limiter.beforeLogin(now), ShouldBeNil)
	})
}
