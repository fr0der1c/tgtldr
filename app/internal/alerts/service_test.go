package alerts

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

type manualClock struct{ now time.Time }

func (c *manualClock) Now() time.Time { return c.now }

type recordingSender struct{ texts []string }

func (s *recordingSender) SendMessageWithLanguage(_ context.Context, _, _, text string, _ model.Language) error {
	s.texts = append(s.texts, text)
	return nil
}

func TestMatchKeyword(t *testing.T) {
	Convey("关键词匹配不区分大小写并跳过空关键词", t, func() {
		keyword, ok := MatchKeyword([]string{" ", "Airdrop", "空投"}, "new AIRDROP is live")
		So(ok, ShouldBeTrue)
		So(keyword, ShouldEqual, "Airdrop")

		keyword, ok = MatchKeyword([]string{"空投"}, "今天有空投活动")
		So(ok, ShouldBeTrue)
		So(keyword, ShouldEqual, "空投")

		_, ok = MatchKeyword([]string{"airdrop"}, "nothing here")
		So(ok, ShouldBeFalse)
	})
}

func TestClaimCooldown(t *testing.T) {
	Convey("同一群组同一关键词在冷却期内只提醒一次", t, func() {
		clk := &manualClock{now: time.Date(2026, time.October, 11, 8, 0, 0, 0, time.UTC)}
		service := NewService(nil, clk, &recordingSender{})

		So(claimed(service, 1, "Airdrop"), ShouldBeTrue)
		So(claimed(service, 1, "airdrop"), ShouldBeFalse)
		So(claimed(service, 2, "airdrop"), ShouldBeTrue)
		So(claimed(service, 1, "other"), ShouldBeTrue)

		clk.now = clk.now.Add(Cooldown)
		So(claimed(service, 1, "airdrop"), ShouldBeTrue)
	})
}

func TestReleaseFailedClaim(t *testing.T) {
	Convey("未送达的提醒释放冷却期，且不会误删之后的登记", t, func() {
		clk := &manualClock{now: time.Date(2026, time.October, 11, 8, 0, 0, 0, time.UTC)}
		service := NewService(nil, clk, &recordingSender{})

		claimedAt, ok := service.claim(1, "airdrop")
		So(ok, ShouldBeTrue)
		service.release(1, "Airdrop", claimedAt)
		So(claimed(service, 1, "airdrop"), ShouldBeTrue)

		clk.now = clk.now.Add(Cooldown)
		second, ok := service.claim(1, "airdrop")
		So(ok, ShouldBeTrue)
		service.release(1, "airdrop", claimedAt)
		So(claimed(service, 1, "airdrop"), ShouldBeFalse)
		service.release(1, "airdrop", second)
		So(claimed(service, 1, "airdrop"), ShouldBeTrue)
	})
}

func claimed(service *Service, chatID int64, keyword string) bool {
	_, ok := service.claim(chatID, keyword)
	return ok
}

func TestHandleSkipsWithoutMatch(t *testing.T) {
	Convey("未开启提醒或未命中关键词时不读取配置也不发送", t, func() {
		bot := &recordingSender{}
		now := time.Now()
		service := NewService(nil, &manualClock{now: now}, bot)
		message := model.Message{TextContent: "airdrop", MessageTime: now}

		So(service.Handle(context.Background(), model.Chat{ID: 1, AlertKeywords: []string{"airdrop"}}, message), ShouldBeNil)
		So(service.Handle(context.Background(), model.Chat{ID: 1, AlertEnabled: true, AlertKeywords: []string{"other"}}, message), ShouldBeNil)
		So(bot.texts, ShouldBeEmpty)
	})

	Convey("重连后补收的旧消息即使命中关键词也不提醒", t, func() {
		bot := &recordingSender{}
		now := time.Date(2026, time.October, 11, 8, 0, 0, 0, time.UTC)
		service := NewService(nil, &manualClock{now: now}, bot)
		chat := model.Chat{ID: 1, AlertEnabled: true, AlertKeywords: []string{"airdrop"}}
		message := model.Message{TextContent: "airdrop", MessageTime: now.Add(-maxMessageAge - time.Second)}

		So(service.Handle(context.Background(), chat, message), ShouldBeNil)
		So(bot.texts, ShouldBeEmpty)
	})
}

func TestBuildMessage(t *testing.T) {
	Convey("提醒正文包含群名、关键词、发送者并截断过长内容", t, func() {
		chat := model.Chat{Title: "Alpha"}
		message := model.Message{SenderName: "Alice", TextContent: strings.Repeat("长", excerptLimit+10)}

		zh := BuildMessage(chat, message, "长", model.LanguageZhCN)
		So(zh, ShouldStartWith, "**关键词提醒 · Alpha**")
		So(zh, ShouldContainSubstring, "发送者：Alice")
		So(zh, ShouldEndWith, strings.Repeat("长", excerptLimit)+"…")

		en := BuildMessage(chat, model.Message{SenderName: "Bob", Caption: "photo caption"}, "photo", model.LanguageEN)
		So(en, ShouldContainSubstring, "Keyword: photo")
		So(en, ShouldEndWith, "photo caption")
	})
}
