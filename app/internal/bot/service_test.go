package bot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSendLongMessage(t *testing.T) {
	Convey("长 Catch Up 应拆成多条完整 Telegram 请求", t, func() {
		messages := make([]string, 0)
		paths := make([]string, 0)
		chatIDs := make([]string, 0)
		parseModes := make([]string, 0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var payload struct {
				ChatID    string `json:"chat_id"`
				Text      string `json:"text"`
				ParseMode string `json:"parse_mode"`
			}
			_ = json.NewDecoder(req.Body).Decode(&payload)
			paths = append(paths, req.URL.Path)
			chatIDs = append(chatIDs, payload.ChatID)
			parseModes = append(parseModes, payload.ParseMode)
			messages = append(messages, payload.Text)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		body := "## Catch Up\n\n- " + strings.Repeat("完整阶段回顾。", 1400) + "\n\n## 末尾\n\n- 来源已保留"
		err := newService(server.URL).SendLongMessage(
			t.Context(), "test-token", "123", body,
		)

		So(err, ShouldBeNil)
		So(len(messages), ShouldBeGreaterThan, 1)
		for index, message := range messages {
			So(paths[index], ShouldEqual, "/bottest-token/sendMessage")
			So(chatIDs[index], ShouldEqual, "123")
			So(parseModes[index], ShouldEqual, "HTML")
			So(telegramVisibleLength(message), ShouldBeLessThanOrEqualTo, telegramMessageVisibleLimit)
		}
		So(messages[len(messages)-1], ShouldContainSubstring, "来源已保留")
	})

	Convey("中途发送失败应指出具体分片", t, func() {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestCount++
			if requestCount == 2 {
				http.Error(w, "temporary failure", http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		body := strings.Repeat("长内容。", 2400)
		err := newService(server.URL).SendLongMessage(
			t.Context(), "test-token", "123", body,
		)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "part 2 of")
	})
}
