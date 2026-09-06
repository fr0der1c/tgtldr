"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { DailyDigestDrawer } from "@/components/daily-digest-drawer";
import { Modal } from "@/components/modal";
import { Button, StatusPill } from "@/components/ui";
import { useToast } from "@/components/toast";
import { api } from "@/lib/api";
import { notifyBootstrapRefresh } from "@/lib/bootstrap-sync";
import { AppSettings, BotSummaryDeliveryMode, DailyDigest, Summary } from "@/lib/types";

/** DailyDigestExperience 展示每日总览入口并协调历史侧栏。 */
export function DailyDigestExperience({
  botConfigured,
  botReady,
  deliveryMode,
  onChanged,
  onEnabled,
  onOpenSummary,
}: {
  botConfigured: boolean | null;
  botReady: boolean;
  deliveryMode: BotSummaryDeliveryMode;
  onChanged: () => Promise<void>;
  onEnabled: (settings: AppSettings) => void;
  onOpenSummary: (summary: Summary) => void;
}) {
  const [latest, setLatest] = useState<DailyDigest | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [enableDialog, setEnableDialog] = useState<"closed" | "confirm" | "configure">("closed");
  const [enabling, setEnabling] = useState(false);
  const router = useRouter();
  const toast = useToast();
  const showErrorRef = useRef(toast.showError);
  const settingsLoaded = botConfigured !== null;
  const enabled = deliveryMode === "daily_digest" && botReady;

  useEffect(() => {
    showErrorRef.current = toast.showError;
  }, [toast.showError]);

  const loadLatest = useCallback(async () => {
    try {
      const response = await api.listDailyDigests(1, 1);
      setLatest(response.items[0] ?? null);
    } catch (error) {
      showErrorRef.current(asMessage(error));
    }
  }, []);

  useEffect(() => {
    void loadLatest();
  }, [loadLatest]);

  /** 打开确认弹窗；Bot 配置不完整时改为引导用户完成设置。 */
  function openEnableDialog() {
    if (botConfigured === null) {
      return;
    }
    setEnableDialog(botConfigured ? "confirm" : "configure");
  }

  /** 同时开启 Bot 推送并将摘要包装方式切换为每日总览。 */
  async function enableDailyDigest() {
    if (enabling) {
      return;
    }
    setEnabling(true);
    try {
      const current = await api.settings();
      if (!hasCompleteBotConfiguration(current)) {
        setEnableDialog("configure");
        return;
      }
      const saved = await api.saveSettings({
        ...current,
        botEnabled: true,
        botSummaryDeliveryMode: "daily_digest",
      });
      onEnabled(saved);
      notifyBootstrapRefresh();
      setEnableDialog("closed");
      toast.showSuccess("每日总览已启用，将从今天的消息开始生效。");
    } catch (error) {
      toast.showError(asMessage(error));
    } finally {
      setEnabling(false);
    }
  }

  function closeEnableDialog() {
    if (!enabling) {
      setEnableDialog("closed");
    }
  }

  function openBotSettings() {
    setEnableDialog("closed");
    router.push("/dashboard/settings?tab=bot");
  }

  return (
    <>
      <section className="catch-up-cta daily-digest-cta">
        <div>
          <span>每日总览</span>
          <h2>一天一篇，看完所有群</h2>
          <p>将所有参与推送的群组摘要合并成一篇，每天只接收一次 Telegram 推送。</p>
          <div className="daily-digest-cta-status">
            {settingsLoaded ? (
              <StatusPill tone={enabled ? "good" : "neutral"}>
                {enabled ? "已启用" : "未启用"}
              </StatusPill>
            ) : (
              <span aria-hidden="true" className="daily-digest-status-placeholder" />
            )}
            {settingsLoaded && enabled && latest ? <small>最近一次：{latest.summaryDate}</small> : null}
            {settingsLoaded && !enabled && latest ? (
              <button className="text-link-button daily-digest-history-link" onClick={() => setDrawerOpen(true)} type="button">
                查看历史总览
              </button>
            ) : null}
          </div>
        </div>
        <Button
          aria-label={settingsLoaded ? undefined : "正在读取每日总览状态"}
          disabled={!settingsLoaded}
          onClick={enabled ? () => setDrawerOpen(true) : openEnableDialog}
          type="button"
        >
          {settingsLoaded ? (
            enabled ? "查看每日总览" : "启用每日总览"
          ) : (
            <span aria-hidden="true" className="daily-digest-button-placeholder">启用每日总览</span>
          )}
        </Button>
      </section>

      <Modal
        actions={(
          <div className="daily-digest-modal-actions">
            <Button disabled={enabling} onClick={closeEnableDialog} type="button" variant="secondary">
              暂不开启
            </Button>
            <Button disabled={enabling} onClick={() => void enableDailyDigest()} type="button">
              {enabling ? "正在启用…" : "确认启用"}
            </Button>
          </div>
        )}
        description="有可汇总内容时，每天只会收到一篇跨群总览。"
        onClose={closeEnableDialog}
        open={enableDialog === "confirm"}
        title="启用每日总览？"
      >
        <div className="daily-digest-enable-copy">
          <p>
            启用时，所有已开启 AI 总结的群组（包括仅网页查看的群组）都会自动参与推送。系统会等待各群摘要完成，合并成一篇「每日总览」发送到 Telegram。之后可在群组设置中单独取消参与。
          </p>
          {!botReady ? (
            <div className="daily-digest-enable-note">
              <strong>同时启用 Telegram Bot</strong>
              <span>Telegram Bot 当前未启用，确认后会一并开启。</span>
            </div>
          ) : null}
          <p className="muted">设置从今天的消息开始生效，之前的摘要不会改变。</p>
        </div>
      </Modal>

      <Modal
        actions={(
          <div className="daily-digest-modal-actions">
            <Button onClick={closeEnableDialog} type="button" variant="secondary">
              取消
            </Button>
            <Button onClick={openBotSettings} type="button">
              前往 Bot 设置
            </Button>
          </div>
        )}
        description="每日总览需要 Telegram Bot 才能发送。"
        onClose={closeEnableDialog}
        open={enableDialog === "configure"}
        title="先完成 Bot 配置"
      >
        <p className="muted">请先配置 Bot Token 和目标 Chat ID，完成后再回来启用每日总览。</p>
      </Modal>

      <DailyDigestDrawer
        botReady={botReady}
        onChanged={async () => {
          await Promise.all([loadLatest(), onChanged()]);
        }}
        onClose={() => {
          setDrawerOpen(false);
          void loadLatest();
        }}
        onOpenSummary={onOpenSummary}
        open={drawerOpen}
      />
    </>
  );
}

function hasCompleteBotConfiguration(settings: AppSettings) {
  return Boolean(settings.botToken?.trim() && settings.botTargetChatId?.trim());
}

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
