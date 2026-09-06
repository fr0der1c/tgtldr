package store

import (
	"context"
	"fmt"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepository struct {
	pool   *pgxpool.Pool
	cipher Cipher
}

func normalizeAppSettings(settings model.AppSettings) model.AppSettings {
	if settings.OpenAIBaseURL == "" {
		settings.OpenAIBaseURL = model.DefaultOpenAIBaseURL
	}
	if settings.SummaryRetryBackoffBaseMinutes <= 0 {
		settings.SummaryRetryBackoffBaseMinutes = model.DefaultSummaryRetryBackoffBaseMinutes
	}
	if settings.SummaryRetryBackoffMultiplier <= 0 {
		settings.SummaryRetryBackoffMultiplier = model.DefaultSummaryRetryBackoffMultiplier
	}
	if settings.OpenAIRequestMode != model.OpenAIRequestModeNonStream {
		settings.OpenAIRequestMode = model.OpenAIRequestModeStream
	}
	if settings.OpenAIContextWindowMode != model.ContextWindowModeManual {
		settings.OpenAIContextWindowMode = model.ContextWindowModeAuto
	}
	settings.Language = model.NormalizeLanguage(settings.Language)
	settings.BotSummaryDeliveryMode = model.NormalizeBotSummaryDeliveryMode(settings.BotSummaryDeliveryMode)
	settings.PreviousBotSummaryDeliveryMode = model.NormalizeBotSummaryDeliveryMode(settings.PreviousBotSummaryDeliveryMode)
	if settings.BotSummaryDeliveryModeEffectiveDate == "" {
		settings.BotSummaryDeliveryModeEffectiveDate = "1970-01-01"
	}
	return settings
}

func (r *SettingsRepository) Get(ctx context.Context) (model.AppSettings, error) {
	var row model.AppSettings
	var encAPIHash string
	var encOpenAIKey string
	var encBotToken string

	err := r.pool.QueryRow(ctx, `
		select id, telegram_api_id, telegram_api_hash, openai_base_url, openai_api_key,
		       openai_model, openai_temperature, openai_output_mode, openai_max_output_tokens,
		       openai_context_window_mode, openai_context_window_tokens, openai_request_mode,
		       summary_parallelism, summary_retry_limit, summary_retry_backoff_base_minutes,
		       summary_retry_backoff_multiplier, default_timezone, language, auto_download_attachments,
		       bot_summary_delivery_mode, previous_bot_summary_delivery_mode,
		       bot_summary_delivery_mode_effective_date::text, bot_enabled,
		       bot_token, bot_target_chat_id, created_at, updated_at
		from app_settings
		order by id
		limit 1
	`).Scan(
		&row.ID,
		&row.TelegramAPIID,
		&encAPIHash,
		&row.OpenAIBaseURL,
		&encOpenAIKey,
		&row.OpenAIModel,
		&row.OpenAITemperature,
		&row.OpenAIOutputMode,
		&row.OpenAIMaxOutputToken,
		&row.OpenAIContextWindowMode,
		&row.OpenAIContextWindowTokens,
		&row.OpenAIRequestMode,
		&row.SummaryParallelism,
		&row.SummaryRetryLimit,
		&row.SummaryRetryBackoffBaseMinutes,
		&row.SummaryRetryBackoffMultiplier,
		&row.DefaultTimezone,
		&row.Language,
		&row.AutoDownloadAttachments,
		&row.BotSummaryDeliveryMode,
		&row.PreviousBotSummaryDeliveryMode,
		&row.BotSummaryDeliveryModeEffectiveDate,
		&row.BotEnabled,
		&encBotToken,
		&row.BotTargetChatID,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return model.AppSettings{}, fmt.Errorf("query settings: %w", err)
	}

	if row.TelegramAPIHash, err = r.cipher.DecryptString(encAPIHash); err != nil {
		return model.AppSettings{}, err
	}
	if row.OpenAIAPIKey, err = r.cipher.DecryptString(encOpenAIKey); err != nil {
		return model.AppSettings{}, err
	}
	if row.BotToken, err = r.cipher.DecryptString(encBotToken); err != nil {
		return model.AppSettings{}, err
	}
	return normalizeAppSettings(row), nil
}

// Save 在启用每日总览时一并纳入已开启 AI 总结的群组，配置与参与范围原子提交。
func (r *SettingsRepository) Save(ctx context.Context, settings model.AppSettings) (model.AppSettings, error) {
	settings = normalizeAppSettings(settings)

	encAPIHash, err := r.cipher.EncryptString(settings.TelegramAPIHash)
	if err != nil {
		return model.AppSettings{}, err
	}
	encOpenAIKey, err := r.cipher.EncryptString(settings.OpenAIAPIKey)
	if err != nil {
		return model.AppSettings{}, err
	}
	encBotToken, err := r.cipher.EncryptString(settings.BotToken)
	if err != nil {
		return model.AppSettings{}, err
	}

	var saved model.AppSettings
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.AppSettings{}, fmt.Errorf("begin settings save: %w", err)
	}
	defer tx.Rollback(ctx)
	var wasEnabled bool
	if err := tx.QueryRow(ctx, `select bot_enabled and bot_summary_delivery_mode = 'daily_digest'
		from app_settings order by id limit 1 for update`).Scan(&wasEnabled); err != nil {
		return model.AppSettings{}, fmt.Errorf("lock settings: %w", err)
	}
	err = tx.QueryRow(ctx, `
		update app_settings
		set telegram_api_id = $1,
		    telegram_api_hash = $2,
		    openai_base_url = $3,
		    openai_api_key = $4,
		    openai_model = $5,
		    openai_temperature = $6,
		    openai_output_mode = $7,
		    openai_max_output_tokens = $8,
		    openai_context_window_mode = $9,
		    openai_context_window_tokens = $10,
		    openai_request_mode = $11,
		    summary_parallelism = $12,
		    summary_retry_limit = $13,
		    summary_retry_backoff_base_minutes = $14,
		    summary_retry_backoff_multiplier = $15,
		    default_timezone = $16,
		    language = $17,
		    auto_download_attachments = $18,
		    bot_summary_delivery_mode = $19,
		    previous_bot_summary_delivery_mode = $20,
		    bot_summary_delivery_mode_effective_date = $21::date,
		    bot_enabled = $22,
		    bot_token = $23,
		    bot_target_chat_id = $24,
		    updated_at = now()
		where id = (select id from app_settings order by id limit 1)
		returning id, created_at, updated_at
	`,
		settings.TelegramAPIID,
		encAPIHash,
		settings.OpenAIBaseURL,
		encOpenAIKey,
		settings.OpenAIModel,
		settings.OpenAITemperature,
		settings.OpenAIOutputMode,
		settings.OpenAIMaxOutputToken,
		settings.OpenAIContextWindowMode,
		settings.OpenAIContextWindowTokens,
		settings.OpenAIRequestMode,
		settings.SummaryParallelism,
		settings.SummaryRetryLimit,
		settings.SummaryRetryBackoffBaseMinutes,
		settings.SummaryRetryBackoffMultiplier,
		settings.DefaultTimezone,
		settings.Language,
		settings.AutoDownloadAttachments,
		settings.BotSummaryDeliveryMode,
		settings.PreviousBotSummaryDeliveryMode,
		settings.BotSummaryDeliveryModeEffectiveDate,
		settings.BotEnabled,
		encBotToken,
		settings.BotTargetChatID,
	).Scan(&saved.ID, &saved.CreatedAt, &saved.UpdatedAt)
	if err != nil {
		return model.AppSettings{}, fmt.Errorf("save settings: %w", err)
	}
	if !wasEnabled && settings.BotEnabled && settings.BotSummaryDeliveryMode == model.BotSummaryDeliveryModeDailyDigest {
		if _, err := tx.Exec(ctx, `update chats set delivery_mode = 'bot', updated_at = now()
			where summary_enabled and delivery_mode <> 'bot'`); err != nil {
			return model.AppSettings{}, fmt.Errorf("enable daily digest participants: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.AppSettings{}, fmt.Errorf("commit settings: %w", err)
	}

	saved.TelegramAPIID = settings.TelegramAPIID
	saved.TelegramAPIHash = settings.TelegramAPIHash
	saved.OpenAIBaseURL = settings.OpenAIBaseURL
	saved.OpenAIAPIKey = settings.OpenAIAPIKey
	saved.OpenAIModel = settings.OpenAIModel
	saved.OpenAITemperature = settings.OpenAITemperature
	saved.OpenAIOutputMode = settings.OpenAIOutputMode
	saved.OpenAIMaxOutputToken = settings.OpenAIMaxOutputToken
	saved.OpenAIContextWindowMode = settings.OpenAIContextWindowMode
	saved.OpenAIContextWindowTokens = settings.OpenAIContextWindowTokens
	saved.OpenAIRequestMode = settings.OpenAIRequestMode
	saved.SummaryParallelism = settings.SummaryParallelism
	saved.SummaryRetryLimit = settings.SummaryRetryLimit
	saved.SummaryRetryBackoffBaseMinutes = settings.SummaryRetryBackoffBaseMinutes
	saved.SummaryRetryBackoffMultiplier = settings.SummaryRetryBackoffMultiplier
	saved.DefaultTimezone = settings.DefaultTimezone
	saved.Language = settings.Language
	saved.AutoDownloadAttachments = settings.AutoDownloadAttachments
	saved.BotSummaryDeliveryMode = settings.BotSummaryDeliveryMode
	saved.PreviousBotSummaryDeliveryMode = settings.PreviousBotSummaryDeliveryMode
	saved.BotSummaryDeliveryModeEffectiveDate = settings.BotSummaryDeliveryModeEffectiveDate
	saved.BotEnabled = settings.BotEnabled
	saved.BotToken = settings.BotToken
	saved.BotTargetChatID = settings.BotTargetChatID
	return saved, nil
}
