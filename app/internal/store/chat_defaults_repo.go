package store

import (
	"context"
	"fmt"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// GetChatDefaults 读取新群组默认配置。
func (r *SettingsRepository) GetChatDefaults(ctx context.Context) (model.ChatDefaults, error) {
	var defaults model.ChatDefaults
	err := r.pool.QueryRow(ctx, `
		select default_chat_delivery_mode, default_chat_summary_time_local,
		       default_chat_model_override, default_chat_keep_bot_messages
		from app_settings
		order by id
		limit 1
	`).Scan(&defaults.DeliveryMode, &defaults.SummaryTimeLocal, &defaults.ModelOverride, &defaults.KeepBotMessages)
	if err != nil {
		return model.ChatDefaults{}, fmt.Errorf("query chat defaults: %w", err)
	}
	return defaults, nil
}

// SaveChatDefaults 只更新新群组默认配置，不影响其他系统设置。
func (r *SettingsRepository) SaveChatDefaults(ctx context.Context, defaults model.ChatDefaults) error {
	_, err := r.pool.Exec(ctx, `
		update app_settings
		set default_chat_delivery_mode = $1,
		    default_chat_summary_time_local = $2,
		    default_chat_model_override = $3,
		    default_chat_keep_bot_messages = $4,
		    updated_at = now()
		where id = (select id from app_settings order by id limit 1)
	`, defaults.DeliveryMode, defaults.SummaryTimeLocal, defaults.ModelOverride, defaults.KeepBotMessages)
	if err != nil {
		return fmt.Errorf("save chat defaults: %w", err)
	}
	return nil
}

// ApplyDefaults 将默认配置覆盖到全部已有群组，返回受影响的群组数量。
func (r *ChatRepository) ApplyDefaults(ctx context.Context, defaults model.ChatDefaults) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		update chats
		set delivery_mode = $1,
		    summary_time_local = $2,
		    model_override = $3,
		    keep_bot_messages = $4,
		    updated_at = now()
	`, defaults.DeliveryMode, defaults.SummaryTimeLocal, defaults.ModelOverride, defaults.KeepBotMessages)
	if err != nil {
		return 0, fmt.Errorf("apply chat defaults: %w", err)
	}
	return tag.RowsAffected(), nil
}
