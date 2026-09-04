"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { DailyDigestDrawer } from "@/components/daily-digest-drawer";
import { Button, StatusPill } from "@/components/ui";
import { useToast } from "@/components/toast";
import { api } from "@/lib/api";
import { BotSummaryDeliveryMode, DailyDigest, Summary } from "@/lib/types";

/** DailyDigestExperience 展示每日总览入口并协调历史侧栏。 */
export function DailyDigestExperience({
  botReady,
  deliveryMode,
  onOpenSummary,
}: {
  botReady: boolean;
  deliveryMode: BotSummaryDeliveryMode;
  onOpenSummary: (summary: Summary) => void;
}) {
  const [latest, setLatest] = useState<DailyDigest | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const toast = useToast();
  const showErrorRef = useRef(toast.showError);
  const configured = deliveryMode === "daily_digest";
  const enabled = configured && botReady;

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

  return (
    <>
      <section className="catch-up-cta daily-digest-cta">
        <div>
          <span>每日总览</span>
          <h2>一天一篇，看完所有群</h2>
          <p>
            {enabled
              ? "参与推送的群组全部完成后，会合并成一篇总览发送到 Telegram。"
              : configured
                ? "每日总览已选择，配置并启用 Bot 后才会开始生成。"
                : "在 Bot 推送设置中开启后，可将多个群组摘要合并为一篇。"}
          </p>
          <div className="daily-digest-cta-status">
            <StatusPill tone={enabled ? "good" : configured ? "warn" : "neutral"}>
              {enabled ? "已启用" : configured ? "等待 Bot 配置" : "未启用"}
            </StatusPill>
            {latest ? <small>最近一次：{latest.summaryDate}</small> : <small>还没有每日总览记录</small>}
          </div>
        </div>
        <Button onClick={() => setDrawerOpen(true)} type="button" variant={enabled ? "primary" : "secondary"}>
          查看每日总览
        </Button>
      </section>

      <DailyDigestDrawer
        botReady={botReady}
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

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
