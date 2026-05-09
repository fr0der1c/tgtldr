package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/frederic/tgtldr/app/internal/httpx"
	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/frederic/tgtldr/app/internal/openai"
)

type openAITestResult struct {
	OK    bool   `json:"ok"`
	Model string `json:"model"`
}

type openAITestValidationError struct {
	zh string
	en string
}

func (e openAITestValidationError) Error() string {
	return e.zh
}

func (r *Router) handleOpenAITest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload model.AppSettings
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	current, err := r.store.Settings.Get(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	settings, err := prepareOpenAITestSettings(payload, current)
	if err != nil {
		var validationErr openAITestValidationError
		if errors.As(err, &validationErr) {
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), validationErr.zh, validationErr.en))
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := runOpenAITest(req.Context(), settings)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func prepareOpenAITestSettings(payload, current model.AppSettings) (model.AppSettings, error) {
	payload.OpenAIAPIKey = preservedSecret(payload.OpenAIAPIKey, current.OpenAIAPIKey)
	if strings.TrimSpace(payload.OpenAIAPIKey) == "" {
		return model.AppSettings{}, openAITestValidationError{
			zh: "请填写 OpenAI API Key。",
			en: "Enter OpenAI API Key.",
		}
	}
	if strings.TrimSpace(payload.OpenAIBaseURL) == "" {
		payload.OpenAIBaseURL = model.DefaultOpenAIBaseURL
	}
	if strings.TrimSpace(payload.OpenAIModel) == "" {
		return model.AppSettings{}, openAITestValidationError{
			zh: "请填写 Model。",
			en: "Enter Model.",
		}
	}
	if payload.OpenAITemperature < 0 || payload.OpenAITemperature > 2 {
		return model.AppSettings{}, openAITestValidationError{
			zh: "Temperature 必须在 0.0 到 2.0 之间。",
			en: "Temperature must be between 0.0 and 2.0.",
		}
	}
	return payload, nil
}

func runOpenAITest(ctx context.Context, settings model.AppSettings) (openAITestResult, error) {
	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   settings.OpenAIModel,
	})
	resp, err := client.Chat(ctx, openai.ChatRequest{
		SystemPrompt: "Reply with exactly OK.",
		UserPrompt:   "Connection test. Reply with exactly OK.",
		Temperature:  settings.OpenAITemperature,
		MaxOutput:    16,
	})
	if err != nil {
		return openAITestResult{}, err
	}
	return openAITestResult{
		OK:    true,
		Model: strings.TrimSpace(resp.Model),
	}, nil
}
