"use client";

import { startTransition, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { AppSelect } from "@/components/app-select";
import { Surface } from "@/components/dashboard-page";
import { useToast } from "@/components/toast";
import { Button, Field, Input } from "@/components/ui";
import { BotSummaryDeliveryMode, ChatDefaults, DeliveryMode } from "@/lib/types";

// ChatDefaultsSurface 编辑新群组默认配置，并支持确认后覆盖到全部已有群组。
export function ChatDefaultsSurface({
  botSummaryDeliveryMode,
  chatCount,
  onApplied
}: {
  botSummaryDeliveryMode: BotSummaryDeliveryMode;
  chatCount: number;
  onApplied: (applied: ChatDefaults) => Promise<void> | void;
}) {
  const [defaults, setDefaults] = useState<ChatDefaults | null>(null);
  const [confirmingApply, setConfirmingApply] = useState(false);
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  const showErrorRef = useRef(toast.showError);

  useEffect(() => {
    showErrorRef.current = toast.showError;
  }, [toast.showError]);

  useEffect(() => {
    let active = true;
    api
      .chatDefaults()
      .then((value) => {
        if (active) setDefaults(value);
      })
      .catch((err) => showErrorRef.current(err instanceof Error ? err.message : String(err)));
    return () => {
      active = false;
    };
  }, []);

  if (!defaults) {
    return null;
  }

  async function save(current: ChatDefaults) {
    setBusy(true);
    try {
      setDefaults(await api.saveChatDefaults(current));
      toast.showSuccess("默认配置已保存。");
    } catch (err) {
      toast.showError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function apply(current: ChatDefaults) {
    setBusy(true);
    try {
      const saved = await api.saveChatDefaults(current);
      setDefaults(saved);
      const result = await api.applyChatDefaults();
      toast.showSuccess(`已应用到 ${result.updated} 个群组。`);
      setConfirmingApply(false);
      await onApplied(saved);
    } catch (err) {
      toast.showError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const patch = (next: Partial<ChatDefaults>) => {
    setDefaults({ ...defaults, ...next });
    setConfirmingApply(false);
  };

  return (
    <Surface
      title="新群组默认配置"
      description="同步到新群组时会套用这些设置；已有群组不受影响，除非点击「应用到所有群组」。"
    >
      <div className="form-grid">
        <Field label="Telegram 推送">
          <AppSelect
            onChange={(value) => patch({ deliveryMode: value as DeliveryMode })}
            options={[
              { value: "dashboard", label: "不参与，仅网页查看" },
              { value: "bot", label: botSummaryDeliveryMode === "daily_digest" ? "纳入每日总览" : "参与推送" }
            ]}
            value={defaults.deliveryMode}
          />
        </Field>

        <Field label="摘要时间">
          <Input
            value={defaults.summaryTimeLocal}
            onChange={(event) => patch({ summaryTimeLocal: event.target.value })}
          />
        </Field>

        <Field label="模型 override">
          <Input
            placeholder="例如 gpt-4.1-mini"
            value={defaults.modelOverride}
            onChange={(event) => patch({ modelOverride: event.target.value })}
          />
        </Field>

        <Field label="保留机器人消息">
          <AppSelect
            onChange={(value) => patch({ keepBotMessages: value === "yes" })}
            options={[
              { value: "yes", label: "保留" },
              { value: "no", label: "过滤" }
            ]}
            value={defaults.keepBotMessages ? "yes" : "no"}
          />
        </Field>
      </div>

      <div className="editor-footer">
        {confirmingApply ? (
          <>
            <p className="muted">{`将覆盖全部 ${chatCount} 个群组的以上四项设置。`}</p>
            <Button disabled={busy} onClick={() => setConfirmingApply(false)} type="button" variant="secondary">
              取消
            </Button>
            <Button disabled={busy} onClick={() => startTransition(() => void apply(defaults))} type="button">
              确认应用
            </Button>
          </>
        ) : (
          <>
            <Button disabled={busy || chatCount === 0} onClick={() => setConfirmingApply(true)} type="button" variant="secondary">
              应用到所有群组
            </Button>
            <Button disabled={busy} onClick={() => startTransition(() => void save(defaults))} type="button">
              保存默认配置
            </Button>
          </>
        )}
      </div>
    </Surface>
  );
}
