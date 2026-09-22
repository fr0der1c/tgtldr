package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

// notifyFailures 等所有来源和总览结束重试后，按日期生成一条去重通知。
func (s *Service) notifyFailures(ctx context.Context, origin string) error {
	settings, err := s.store.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if !settings.BotEnabled || settings.BotToken == "" || settings.BotTargetChatID == "" {
		return nil
	}
	dates, err := s.store.FailureDates(ctx)
	if err != nil {
		return err
	}
	for _, date := range dates {
		states, err := s.store.FailureStates(ctx, date)
		if err != nil {
			return err
		}
		dailyDigest := model.ResolveBotSummaryDeliveryMode(settings, date) == model.BotSummaryDeliveryModeDailyDigest
		message, ready := failureMessage(date, states, settings.SummaryRetryLimit, settings.Language, origin, dailyDigest)
		if !ready {
			continue
		}
		if err = s.store.QueueFailureNotice(ctx, date, message); err != nil {
			return err
		}
	}
	notice, err := s.store.ClaimFailureNotice(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	sendErr := s.botService.SendMessageWithLanguage(ctx, settings.BotToken, settings.BotTargetChatID, notice.Message, settings.Language)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.store.FinishFailureNotice(ctx, notice.Date, failureDeliveryError(sendErr))
}

// failureDeliveryError 仅记录受控原因代码，不记录可能含有 Bot Token 的请求 URL 或响应。
func failureDeliveryError(err error) string {
	if err == nil {
		return ""
	}
	var deliveryErr *bot.DeliveryError
	if errors.As(err, &deliveryErr) {
		switch deliveryErr.StatusCode {
		case 400:
			return "request"
		case 401:
			return "auth"
		case 403:
			return "permission"
		case 429:
			return "rate_limit"
		}
		if deliveryErr.StatusCode >= 500 {
			return "upstream"
		}
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return "timeout"
	}
	return "delivery_failed"
}

// failureMessage 只展示受控错误分类，不转发上游响应、提示词、密钥或聊天正文。
func failureMessage(date string, states []store.FailureState, limit int, language model.Language, origin string, dailyDigest bool) (string, bool) {
	failed := 0
	digestFailed := false
	digestAffected := false
	reasons := map[string]bool{}
	for _, state := range states {
		if state.Status == "pending" || state.Status == "running" {
			return "", false
		}
		if state.Status != "failed" {
			continue
		}
		if state.NextRetryAt != nil && state.RetryCount < limit {
			return "", false
		}
		if state.Kind == "digest" {
			digestFailed = true
		} else {
			failed++
			digestAffected = digestAffected || state.InDigest
		}
		reasons[failureReason(state.Error, language)] = true
	}
	if failed == 0 && !digestFailed {
		return "", false
	}
	labels := []string{}
	for reason := range reasons {
		labels = append(labels, reason)
	}
	sort.Strings(labels)
	impact := "这些群组的摘要暂时不可用。"
	if dailyDigest && digestAffected {
		impact = "每日总览可能缺少这些群组内容。"
	}
	if digestFailed {
		impact = "每日总览生成失败，无法推送。"
	}
	message := fmt.Sprintf("**摘要生成失败 · %s**\n\n%d 个群组摘要失败，自动重试已结束。\n原因：%s\n\n%s请检查摘要引擎配置，然后使用“一键重跑失败任务”。", date, failed, strings.Join(labels, "、"), impact)
	if language == model.LanguageEN {
		impact = "These chat summaries are currently unavailable."
		if dailyDigest && digestAffected {
			impact = "The Daily Digest may omit these chats."
		}
		if digestFailed {
			impact = "The Daily Digest failed to generate and cannot be sent."
		}
		message = fmt.Sprintf("**Summary generation failed · %s**\n\n%d chat summaries failed. Automatic retries have ended.\nReason: %s\n\n%s Check the summary engine settings, then use Retry all failed tasks.", date, failed, strings.Join(labels, ", "), impact)
	}
	if strings.HasPrefix(origin, "https://") || strings.HasPrefix(origin, "http://") {
		message += "\n\n" + strings.TrimRight(origin, "/") + "/dashboard/summaries"
	}
	return message, true
}

// failureReason 将外部错误归类为有限的中英文提示，避免泄露原始响应内容。
func failureReason(message string, language model.Language) string {
	zh, en := "摘要生成失败，请在网页查看详情", "Generation failed; see details on the dashboard"
	switch {
	case strings.Contains(message, "model not found"):
		zh, en = "模型不存在或不可用", "Model not found or unavailable"
	case strings.Contains(message, "status 401"), strings.Contains(message, "status 403"):
		zh, en = "模型服务认证或权限错误", "Model service authentication or permission error"
	case strings.Contains(message, "status 429"):
		zh, en = "模型服务限流或额度不足", "Model service rate limit or quota exceeded"
	case strings.Contains(message, "status 5"):
		zh, en = "模型服务暂时不可用", "Model service temporarily unavailable"
	case strings.Contains(message, "deadline exceeded"), strings.Contains(message, "timeout"):
		zh, en = "模型请求超时", "Model request timed out"
	}
	if language == model.LanguageEN {
		return en
	}
	return zh
}
