package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/catchup"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/localauth"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/scheduler"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	telegramsvc "github.com/fr0der1c/tgtldr/app/internal/telegram"
	"github.com/jackc/pgx/v5"
)

type Router struct {
	store     *store.Store
	bot       *bot.Service
	telegram  *telegramsvc.Service
	scheduler *scheduler.Service
	catchups  *catchup.Service
	auth      *localauth.Service
	origin    string
	timeout   time.Duration
	mediaDir  string
}

const chatMessageActivityDays = 30

func New(
	store *store.Store,
	telegram *telegramsvc.Service,
	scheduler *scheduler.Service,
	catchups *catchup.Service,
	botService *bot.Service,
	origin string,
	timeout time.Duration,
	mediaDir string,
) *Router {
	return &Router{
		store:     store,
		bot:       botService,
		telegram:  telegram,
		scheduler: scheduler,
		catchups:  catchups,
		auth:      localauth.NewService(store),
		origin:    origin,
		timeout:   timeout,
		mediaDir:  mediaDir,
	}
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", r.handleHealth)
	mux.HandleFunc("/api/bootstrap", r.handleBootstrap)
	mux.HandleFunc("/api/auth/login", r.handleLogin)
	mux.HandleFunc("/api/auth/logout", r.handleLogout)
	mux.HandleFunc("/api/auth/setup-password", r.handleSetupPassword)
	mux.HandleFunc("/api/auth/change-password", r.handleChangePassword)
	mux.HandleFunc("/api/settings", r.handleSettings)
	mux.HandleFunc("/api/settings/openai-test", r.handleOpenAITest)
	mux.HandleFunc("/api/bot/target-chat/resolve", r.handleResolveBotTargetChat)
	mux.HandleFunc("/api/telegram/auth/start", r.handleStartAuth)
	mux.HandleFunc("/api/telegram/auth/code", r.handleVerifyCode)
	mux.HandleFunc("/api/telegram/auth/password", r.handleVerifyPassword)
	mux.HandleFunc("/api/telegram/auth/cancel", r.handleCancelTelegramAuth)
	mux.HandleFunc("/api/telegram/accounts", r.handleTelegramAccounts)
	mux.HandleFunc("/api/telegram/accounts/", r.handleTelegramAccountByID)
	mux.HandleFunc("/api/telegram/chats/sync", r.handleSyncChats)
	mux.HandleFunc("/api/chats", r.handleChats)
	mux.HandleFunc("GET /api/messages/search", r.handleGlobalMessageSearch)
	mux.HandleFunc("GET /api/chats/{chatID}/messages/search", r.handleChatMessageSearch)
	mux.HandleFunc("GET /api/chats/{chatID}/messages", r.handleChatMessages)
	mux.HandleFunc("GET /api/assets/{assetID}/content", r.handleAssetContent)
	mux.HandleFunc("HEAD /api/assets/{assetID}/content", r.handleAssetContent)
	mux.HandleFunc("POST /api/assets/{assetID}/download", r.handleAssetDownload)
	mux.HandleFunc("/api/chats/", r.handleChatByID)
	mux.HandleFunc("/api/history-backfills", r.handleStartHistoryBackfill)
	mux.HandleFunc("/api/history-backfills/", r.handleHistoryBackfillByID)
	mux.HandleFunc("/api/summaries", r.handleSummaries)
	mux.HandleFunc("/api/summaries/stats", r.handleSummaryStats)
	mux.HandleFunc("/api/summaries/", r.handleSummaryByID)
	mux.HandleFunc("/api/summaries/context-preview", r.handleSummaryContextPreview)
	mux.HandleFunc("/api/summaries/run", r.handleRunSummary)
	mux.HandleFunc("/api/catch-ups", r.handleCatchUps)
	mux.HandleFunc("/api/catch-ups/", r.handleCatchUpByID)

	return r.withMiddleware(mux)
}

func (r *Router) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if origin := allowedOrigin(req.Header.Get("Origin"), r.origin); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ctx, cancel := context.WithTimeout(req.Context(), r.timeout)
		defer cancel()
		authorizedReq, ok := r.authorizeRequest(w, req.WithContext(ctx))
		if !ok {
			return
		}
		next.ServeHTTP(w, authorizedReq)
	})
}

func allowedOrigin(requestOrigin, configuredOrigin string) string {
	switch requestOrigin {
	case "", configuredOrigin, "http://localhost:3000", "http://127.0.0.1:3000":
		if requestOrigin != "" {
			return requestOrigin
		}
		return configuredOrigin
	default:
		return ""
	}
}

func preservedSecret(incoming string, current string) string {
	trimmed := strings.TrimSpace(incoming)
	if trimmed == "" {
		return current
	}

	sanitized := redactSecret(current)
	if sanitized != "" && trimmed == sanitized {
		return current
	}
	return incoming
}

func redactSecret(secret string) string {
	if len(secret) <= 4 {
		return ""
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}

func settingsConfigured(settings model.AppSettings) bool {
	return settings.TelegramAPIID != 0 &&
		strings.TrimSpace(settings.TelegramAPIHash) != "" &&
		strings.TrimSpace(settings.OpenAIAPIKey) != "" &&
		strings.TrimSpace(settings.OpenAIModel) != ""
}

func (r *Router) currentLanguage(ctx context.Context) model.Language {
	settings, err := r.store.Settings.Get(ctx)
	if err != nil {
		return model.LanguageZhCN
	}
	return model.NormalizeLanguage(settings.Language)
}

func (r *Router) localized(ctx context.Context, zh string, en string) string {
	if r.currentLanguage(ctx) == model.LanguageEN {
		return en
	}
	return zh
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleBootstrap(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	passwordConfigured, err := r.auth.PasswordConfigured(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	language := r.currentLanguage(req.Context())
	authenticated := currentSessionID(req.Context()) != ""
	if !authenticated {
		httpx.JSON(w, http.StatusOK, map[string]any{
			"settingsConfigured": false,
			"passwordConfigured": passwordConfigured,
			"authenticated":      false,
			"telegramAuthorized": false,
			"enabledChatCount":   0,
			"botEnabled":         false,
			"language":           language,
		})
		return
	}

	settings, err := r.store.Settings.Get(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	auth, err := r.telegram.BootstrapAuth(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	count, err := r.store.Chats.CountEnabled(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	accounts, err := r.store.Auth.List(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	authorizedCount := 0
	for _, account := range accounts {
		if account.Status == "authorized" {
			authorizedCount++
		}
	}
	payload := map[string]any{
		"settingsConfigured":     settingsConfigured(settings),
		"passwordConfigured":     passwordConfigured,
		"authenticated":          authenticated,
		"telegramAuthorized":     authorizedCount > 0,
		"authorizedAccountCount": authorizedCount,
		"telegramAccounts":       accounts,
		"enabledChatCount":       count,
		"botEnabled":             settings.BotEnabled,
		"language":               model.NormalizeLanguage(settings.Language),
		"settings":               settings.Sanitized(),
		"auth":                   auth,
		"pendingAuth":            r.telegram.PendingAuthState(),
	}
	httpx.JSON(w, http.StatusOK, payload)
}

func (r *Router) handleSettings(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPut && req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if req.Method == http.MethodGet {
		settings, err := r.store.Settings.Get(req.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, settings.Sanitized())
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
	payload.TelegramAPIHash = preservedSecret(payload.TelegramAPIHash, current.TelegramAPIHash)
	payload.OpenAIAPIKey = preservedSecret(payload.OpenAIAPIKey, current.OpenAIAPIKey)
	payload.BotToken = preservedSecret(payload.BotToken, current.BotToken)
	if payload.TelegramAPIID == 0 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请填写 Telegram API ID。", "Enter Telegram API ID."))
		return
	}
	if strings.TrimSpace(payload.TelegramAPIHash) == "" {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请填写 Telegram API Hash。", "Enter Telegram API Hash."))
		return
	}
	if strings.TrimSpace(payload.OpenAIAPIKey) == "" {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请填写 OpenAI API Key。", "Enter OpenAI API Key."))
		return
	}
	if strings.TrimSpace(payload.OpenAIBaseURL) == "" {
		payload.OpenAIBaseURL = model.DefaultOpenAIBaseURL
	}
	if strings.TrimSpace(payload.OpenAIModel) == "" {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请填写 Model。", "Enter Model."))
		return
	}
	if payload.OpenAITemperature < 0 || payload.OpenAITemperature > 2 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "Temperature 必须在 0.0 到 2.0 之间。", "Temperature must be between 0.0 and 2.0."))
		return
	}
	if strings.TrimSpace(payload.DefaultTimezone) == "" {
		payload.DefaultTimezone = "Asia/Shanghai"
	}
	if payload.Language == "" {
		payload.Language = model.LanguageZhCN
	}
	if payload.Language != model.LanguageZhCN && payload.Language != model.LanguageEN {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "语言必须是 zh-CN 或 en。", "Language must be zh-CN or en."))
		return
	}
	if payload.OpenAIOutputMode == "" {
		payload.OpenAIOutputMode = model.OutputModeAuto
	}
	if payload.OpenAIOutputMode != model.OutputModeAuto && payload.OpenAIOutputMode != model.OutputModeManual {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "输出长度模式必须是 auto 或 manual。", "Output length mode must be auto or manual."))
		return
	}
	if payload.OpenAIOutputMode == model.OutputModeManual && payload.OpenAIMaxOutputToken <= 0 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "自定义输出长度时必须填写 Max Output Tokens。", "Max Output Tokens is required when output length is custom."))
		return
	}
	if payload.OpenAIContextWindowMode == "" {
		payload.OpenAIContextWindowMode = model.ContextWindowModeAuto
	}
	if payload.OpenAIContextWindowMode != model.ContextWindowModeAuto && payload.OpenAIContextWindowMode != model.ContextWindowModeManual {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "上下文长度模式必须是 auto 或 manual。", "Context window mode must be auto or manual."))
		return
	}
	if payload.OpenAIContextWindowMode == model.ContextWindowModeManual && payload.OpenAIContextWindowTokens < 4096 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "自定义上下文长度不能小于 4096 tokens。", "Custom context window must be at least 4096 tokens."))
		return
	}
	if payload.OpenAIRequestMode == "" {
		payload.OpenAIRequestMode = model.OpenAIRequestModeStream
	}
	if payload.OpenAIRequestMode != model.OpenAIRequestModeStream && payload.OpenAIRequestMode != model.OpenAIRequestModeNonStream {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "调用方式必须是 stream 或 non_stream。", "Request mode must be stream or non_stream."))
		return
	}
	if payload.SummaryParallelism <= 0 {
		payload.SummaryParallelism = 2
	}
	if payload.SummaryParallelism < 1 || payload.SummaryParallelism > 6 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "摘要并行度必须在 1 到 6 之间。", "Summary parallelism must be between 1 and 6."))
		return
	}
	if payload.SummaryRetryLimit < 0 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "重试次数上限不能小于 0。", "Summary retry limit cannot be less than 0."))
		return
	}
	if payload.SummaryRetryBackoffBaseMinutes < 1 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "重试起始间隔必须至少为 1 分钟。", "Summary retry backoff base must be at least 1 minute."))
		return
	}
	if payload.SummaryRetryBackoffMultiplier < 1 {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "退避倍率必须大于或等于 1。", "Summary retry backoff multiplier must be at least 1."))
		return
	}
	if payload.BotEnabled && strings.TrimSpace(payload.BotToken) == "" {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "启用 Bot 推送时必须填写 Bot Token。", "Bot Token is required when Bot delivery is enabled."))
		return
	}

	saved, err := r.store.Settings.Save(req.Context(), payload)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := r.store.Assets.ApplyAutoDownloadPolicy(req.Context(), saved.AutoDownloadAttachments); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, saved.Sanitized())
}

func (r *Router) handleResolveBotTargetChat(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload struct {
		BotToken          string `json:"botToken"`
		TelegramAccountID int64  `json:"telegramAccountId"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	auth, err := r.botTargetTelegramAccount(req.Context(), payload.TelegramAccountID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if auth == nil || auth.Status != "authorized" {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请先完成 Telegram 登录。", "Complete Telegram login first."))
		return
	}

	botToken := strings.TrimSpace(payload.BotToken)
	if botToken == "" {
		settings, err := r.store.Settings.Get(req.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		botToken = strings.TrimSpace(settings.BotToken)
	}
	if botToken == "" {
		httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请先填写 Bot Token。", "Enter the Bot Token first."))
		return
	}

	candidates, err := r.bot.ResolveTargetChats(req.Context(), botToken, auth.TelegramUserID)
	if err != nil {
		status := http.StatusBadGateway
		var botErr *bot.APIError
		if errors.As(err, &botErr) && botErr.StatusCode >= 400 && botErr.StatusCode < 500 {
			status = http.StatusBadRequest
		}
		httpx.Error(w, status, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"candidates": candidates,
	})
}

func (r *Router) botTargetTelegramAccount(ctx context.Context, accountID int64) (*model.TelegramAuth, error) {
	if accountID == 0 {
		return r.telegram.BootstrapAuth(ctx)
	}
	return r.store.Auth.GetByID(ctx, accountID)
}

func (r *Router) handleStartAuth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		PhoneNumber string `json:"phoneNumber"`
		AccountID   int64  `json:"accountId"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	state, err := r.telegram.StartAuth(req.Context(), payload.PhoneNumber, payload.AccountID)
	if err != nil {
		if floodErr, ok := asFloodWaitError(err); ok {
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, floodErr.Error(), "telegram_flood_wait", floodErr.RetryAfterSeconds())
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, state)
}

func (r *Router) handleCancelTelegramAuth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.telegram.CancelPendingAuth(req.Context()); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleVerifyCode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	state, err := r.telegram.VerifyCode(req.Context(), payload.Code)
	if err != nil {
		if errors.Is(err, telegramsvc.ErrPasswordNeeded) {
			httpx.JSON(w, http.StatusAccepted, state)
			return
		}
		if floodErr, ok := asFloodWaitError(err); ok {
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, floodErr.Error(), "telegram_flood_wait", floodErr.RetryAfterSeconds())
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, state)
}

func (r *Router) handleVerifyPassword(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	state, err := r.telegram.VerifyPassword(req.Context(), payload.Password)
	if err != nil {
		if floodErr, ok := asFloodWaitError(err); ok {
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, floodErr.Error(), "telegram_flood_wait", floodErr.RetryAfterSeconds())
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, state)
}

func (r *Router) handleSyncChats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.telegram.SyncChats(req.Context()); err != nil {
		if floodErr, ok := asFloodWaitError(err); ok {
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, floodErr.Error(), "telegram_flood_wait", floodErr.RetryAfterSeconds())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	chats, err := r.store.Chats.List(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := r.attachAvailableAccounts(req.Context(), chats); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := r.attachChatAvatars(req.Context(), chats); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, chats)
}

func asFloodWaitError(err error) (*telegramsvc.FloodWaitError, bool) {
	var floodErr *telegramsvc.FloodWaitError
	if !errors.As(err, &floodErr) {
		return nil, false
	}
	return floodErr, true
}

func (r *Router) handleChats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	startLocal, timezone, err := r.chatMessageActivityWindow(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	chats, err := r.store.Chats.ListWithMessageActivity(
		req.Context(),
		startLocal,
		chatMessageActivityDays,
		timezone,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := r.attachAvailableAccounts(req.Context(), chats); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := r.attachChatAvatars(req.Context(), chats); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, chats)
}

// attachChatAvatars 为群组列表补充已下载头像地址，未下载时保持空值。
func (r *Router) attachChatAvatars(ctx context.Context, chats []model.Chat) error {
	for index := range chats {
		peerType := "chat"
		if chats[index].ChatType == "supergroup" {
			peerType = "channel"
		}
		asset, err := r.store.Assets.FindEntityAvatar(ctx, chats[index].CollectorAccountID, peerType, chats[index].TelegramChatID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		chats[index].AvatarURL = assetContentURL(asset.ID)
	}
	return nil
}

func (r *Router) attachAvailableAccounts(ctx context.Context, chats []model.Chat) error {
	chatIDs := make([]int64, 0, len(chats))
	for _, chat := range chats {
		chatIDs = append(chatIDs, chat.ID)
	}
	accountsByChat, err := r.store.AccountChats.ListForChats(ctx, chatIDs)
	if err != nil {
		return err
	}
	for i := range chats {
		chats[i].AvailableAccounts = accountsByChat[chats[i].ID]
	}
	return nil
}

func (r *Router) chatMessageActivityWindow(ctx context.Context) (time.Time, string, error) {
	settings, err := r.store.Settings.Get(ctx)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("get settings for chat message activity: %w", err)
	}
	timezone := strings.TrimSpace(settings.DefaultTimezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("load chat message activity timezone %s: %w", timezone, err)
	}

	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return today.AddDate(0, 0, 1-chatMessageActivityDays), timezone, nil
}

func (r *Router) handleChatByID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPut {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idValue := strings.TrimPrefix(req.URL.Path, "/api/chats/")
	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	current, err := r.store.Chats.GetByID(req.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err.Error())
		return
	}

	var payload struct {
		Enabled            bool               `json:"enabled"`
		SummaryEnabled     bool               `json:"summaryEnabled"`
		SummaryContext     string             `json:"summaryContext"`
		SummaryPrompt      string             `json:"summaryPrompt"`
		SummaryTimeLocal   string             `json:"summaryTimeLocal"`
		DeliveryMode       model.DeliveryMode `json:"deliveryMode"`
		ModelOverride      string             `json:"modelOverride"`
		KeepBotMessages    bool               `json:"keepBotMessages"`
		FilteredSenders    []string           `json:"filteredSenders"`
		FilteredKeywords   []string           `json:"filteredKeywords"`
		CollectorAccountID int64              `json:"collectorAccountId"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	current.Enabled = payload.Enabled
	current.SummaryEnabled = payload.SummaryEnabled
	current.SummaryContext = payload.SummaryContext
	current.SummaryPrompt = payload.SummaryPrompt
	current.SummaryTimeLocal = payload.SummaryTimeLocal
	current.DeliveryMode = payload.DeliveryMode
	current.ModelOverride = payload.ModelOverride
	current.KeepBotMessages = payload.KeepBotMessages
	current.FilteredSenders = compactStrings(payload.FilteredSenders)
	current.FilteredKeywords = compactStrings(payload.FilteredKeywords)
	if payload.CollectorAccountID != current.CollectorAccountID {
		if err := r.store.Chats.SetCollectorAccount(req.Context(), current.ID, payload.CollectorAccountID); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		current.CollectorAccountID = payload.CollectorAccountID
	}

	saved, err := r.store.Chats.Save(req.Context(), current)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	saved.AvailableAccounts, err = r.store.AccountChats.ListForChat(req.Context(), saved.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, saved)
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func (r *Router) handleSummaries(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	params := store.SummaryListParams{
		Query:    strings.TrimSpace(req.URL.Query().Get("q")),
		Status:   strings.TrimSpace(req.URL.Query().Get("status")),
		Delivery: strings.TrimSpace(req.URL.Query().Get("delivery")),
		DateFrom: strings.TrimSpace(req.URL.Query().Get("dateFrom")),
		DateTo:   strings.TrimSpace(req.URL.Query().Get("dateTo")),
	}

	if chatIDValue := strings.TrimSpace(req.URL.Query().Get("chatId")); chatIDValue != "" {
		chatID, err := strconv.ParseInt(chatIDValue, 10, 64)
		if err != nil || chatID < 0 {
			httpx.Error(w, http.StatusBadRequest, "invalid chatId")
			return
		}
		params.ChatID = chatID
	}

	if pageValue := strings.TrimSpace(req.URL.Query().Get("page")); pageValue != "" {
		page, err := strconv.Atoi(pageValue)
		if err != nil || page < 1 {
			httpx.Error(w, http.StatusBadRequest, "invalid page")
			return
		}
		params.Page = page
	}

	if pageSizeValue := strings.TrimSpace(req.URL.Query().Get("pageSize")); pageSizeValue != "" {
		pageSize, err := strconv.Atoi(pageSizeValue)
		if err != nil || pageSize < 1 {
			httpx.Error(w, http.StatusBadRequest, "invalid pageSize")
			return
		}
		params.PageSize = pageSize
	}

	summaries, err := r.store.Summaries.Search(req.Context(), params)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, summaries)
}

func (r *Router) handleSummaryStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats, err := r.store.Summaries.Stats(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, stats)
}

func (r *Router) handleSummaryContextPreview(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idValue := strings.TrimSpace(req.URL.Query().Get("id"))
	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil || id == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid summary id")
		return
	}

	item, err := r.store.Summaries.GetByID(req.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err.Error())
		return
	}

	preview, err := r.scheduler.ContextPreview(req.Context(), item)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, preview)
}

func (r *Router) handleRunSummary(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload struct {
		ChatID int64  `json:"chatId"`
		Date   string `json:"date"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	chat, err := r.store.Chats.GetByID(req.Context(), payload.ChatID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err.Error())
		return
	}
	started, err := r.scheduler.RunNowAsync(req.Context(), chat, payload.Date)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	message := fmt.Sprintf("summary queued for chat %d on %s", payload.ChatID, payload.Date)
	if !started {
		message = fmt.Sprintf("summary already running for chat %d on %s", payload.ChatID, payload.Date)
	}

	httpx.JSON(w, http.StatusAccepted, map[string]string{
		"message": message,
	})
}

func (r *Router) handleSummaryByID(w http.ResponseWriter, req *http.Request) {
	trimmed := strings.TrimPrefix(req.URL.Path, "/api/summaries/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 1 && req.Method == http.MethodGet {
		summaryID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || summaryID <= 0 {
			httpx.Error(w, http.StatusBadRequest, "invalid summary id")
			return
		}
		item, err := r.store.Summaries.GetByID(req.Context(), summaryID)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, "summary not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, item)
		return
	}
	if len(parts) != 2 || parts[1] != "retry-delivery" {
		httpx.Error(w, http.StatusNotFound, "not found")
		return
	}
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	summaryID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || summaryID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid summary id")
		return
	}

	if err := r.scheduler.RetryDelivery(req.Context(), summaryID); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("delivery retried for summary %d", summaryID),
	})
}
