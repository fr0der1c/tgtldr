package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frederic/tgtldr/app/internal/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPrepareOpenAITestSettings(t *testing.T) {
	current := model.AppSettings{
		OpenAIAPIKey: "saved-key",
	}

	Convey("空 Base URL 会使用默认值", t, func() {
		settings, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldBeNil)
		So(settings.OpenAIBaseURL, ShouldEqual, model.DefaultOpenAIBaseURL)
	})

	Convey("空 API Key 会保留已保存密钥", t, func() {
		settings, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldBeNil)
		So(settings.OpenAIAPIKey, ShouldEqual, "saved-key")
	})

	Convey("缺少模型会报错", t, func() {
		_, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIAPIKey:      "test-key",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "Model")
	})
}

func TestOpenAITestRouteRegistration(t *testing.T) {
	Convey("OpenAI 测试路由可以和设置路由一起注册", t, func() {
		So(func() {
			_ = (&Router{}).Handler()
		}, ShouldNotPanic)
	})
}

func TestRunOpenAITest(t *testing.T) {
	Convey("测试连接会发送低成本 chat completion 请求", t, func() {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-test","choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
		}))
		defer server.Close()

		result, err := runOpenAITest(context.Background(), model.AppSettings{
			OpenAIBaseURL:     server.URL,
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		})

		So(err, ShouldBeNil)
		So(result.OK, ShouldBeTrue)
		So(result.Model, ShouldEqual, "gpt-test")
		So(payload["max_completion_tokens"], ShouldEqual, float64(16))
	})

	Convey("上游错误会原样返回给调用方", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}))
		defer server.Close()

		_, err := runOpenAITest(context.Background(), model.AppSettings{
			OpenAIBaseURL:     server.URL,
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		})

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "openai status 502")
	})
}
