"use client";

import { useEffect, useRef, useState } from "react";
import { Modal } from "@/components/modal";
import { Button, Input } from "@/components/ui";
import { useToast } from "@/components/toast";
import { api } from "@/lib/api";
import { CatchUp, Chat } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

type WizardStep = "period" | "chats" | "generating";
type PeriodPreset = 7 | 14 | 30 | "custom";

/** Catch UpWizard 管理两步选择、后台任务提交和页面内完成轮询。 */
export function CatchUpWizard({
  botReady,
  chats,
  onClose,
  onCompleted,
  onOpenHistory,
  open,
  timezone,
}: {
  botReady: boolean;
  chats: Chat[];
  onClose: () => void;
  onCompleted: (item: CatchUp, waitedInDialog: boolean) => void;
  onOpenHistory: () => void;
  open: boolean;
  timezone: string;
}) {
  const [step, setStep] = useState<WizardStep>("period");
  const [preset, setPreset] = useState<PeriodPreset>(7);
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [selectedChatIds, setSelectedChatIds] = useState<Set<number>>(new Set());
  const [sendToTelegram, setSendToTelegram] = useState(botReady);
  const [activeTask, setActiveTask] = useState<CatchUp | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const openRef = useRef(open);
  const toast = useToast();
  const { language } = useI18n();

  useEffect(() => {
    openRef.current = open;
  }, [open]);

  useEffect(() => {
    if (!open || activeTask?.status === "pending" || activeTask?.status === "running") {
      return;
    }
    const range = dateRangeForDays(7, timezone);
    setStep("period");
    setPreset(7);
    setFromDate(range.fromDate);
    setToDate(range.toDate);
    setSelectedChatIds(new Set(chats.map((chat) => chat.id)));
    setSendToTelegram(botReady);
    setActiveTask(null);
  }, [activeTask?.status, botReady, chats, open, timezone]);

  useEffect(() => {
    if (!activeTask || (activeTask.status !== "pending" && activeTask.status !== "running")) {
      return;
    }
    let cancelled = false;
    let timer = 0;

    /** 轮询持久化任务；关闭弹窗后仍继续等待当前页面内的完成通知。 */
    async function poll() {
      try {
        const next = await api.getCatchUp(activeTask!.id);
        if (cancelled) return;
        setActiveTask(next);
        if (next.status === "succeeded" || next.status === "failed") {
          onCompleted(next, openRef.current);
          return;
        }
      } catch (error) {
        if (!cancelled) toast.showError(asMessage(error));
      }
      if (!cancelled) timer = window.setTimeout(poll, 2000);
    }

    timer = window.setTimeout(poll, 1200);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [activeTask, onCompleted, toast]);

  /** 应用固定周期，并将截止日设为系统时区中的昨天。 */
  function selectPreset(value: Exclude<PeriodPreset, "custom">) {
    const range = dateRangeForDays(value, timezone);
    setPreset(value);
    setFromDate(range.fromDate);
    setToDate(range.toDate);
  }

  /** 切换单个群组时复制集合，避免直接修改 React 状态。 */
  function toggleChat(chatId: number) {
    setSelectedChatIds((current) => {
      const next = new Set(current);
      if (next.has(chatId)) next.delete(chatId);
      else next.add(chatId);
      return next;
    });
  }

  /** 提交任务后立即进入等待态，后台任务不会依赖当前弹窗生命周期。 */
  async function createCatchUp() {
    if (selectedChatIds.size === 0 || submitting) return;
    setSubmitting(true);
    try {
      const item = await api.createCatchUp({
        fromDate,
        toDate,
        chatIds: Array.from(selectedChatIds),
        sendToTelegram: botReady && sendToTelegram,
      });
      setActiveTask(item);
      setStep("generating");
    } catch (error) {
      toast.showError(asMessage(error));
    } finally {
      setSubmitting(false);
    }
  }

  const periodValid = Boolean(fromDate && toDate && fromDate <= toDate);
  const yesterday = dateRangeForDays(1, timezone).toDate;

  return (
    <Modal
      actions={
        step === "period" ? (
          <Button disabled={!periodValid} onClick={() => setStep("chats")} type="button">
            下一步
          </Button>
        ) : step === "chats" ? (
          <div className="catch-up-modal-actions">
            <Button onClick={() => setStep("period")} type="button" variant="secondary">
              上一步
            </Button>
            <Button
              disabled={selectedChatIds.size === 0 || submitting}
              onClick={() => void createCatchUp()}
              type="button"
            >
              {submitting ? "正在提交…" : "开始生成"}
            </Button>
          </div>
        ) : null
      }
      description="从已有的每日摘要中，快速整理出一份阶段回顾。"
      onClose={onClose}
      open={open}
      title="快速回顾"
    >
      <div className="catch-up-wizard">
        <button className="text-link-button catch-up-history-link" onClick={onOpenHistory} type="button">
          查看之前的快速回顾
        </button>

        {step === "period" ? (
          <section className="catch-up-step">
            <div className="catch-up-step-heading">
              <span>第 1 步，共 2 步</span>
              <h3>选择时间周期</h3>
              <p>固定周期均截止到昨天，不会混入尚未结束的今天。</p>
            </div>
            <div className="catch-up-period-options">
              {([7, 14, 30] as const).map((days) => (
                <button
                  aria-pressed={preset === days}
                  className={`catch-up-period-option${preset === days ? " selected" : ""}`}
                  key={days}
                  onClick={() => selectPreset(days)}
                  type="button"
                >
                  <strong>{language === "en" ? `${days} days` : `${days} 天`}</strong>
                  <span>{formatRange(dateRangeForDays(days, timezone))}</span>
                </button>
              ))}
              <button
                aria-pressed={preset === "custom"}
                className={`catch-up-period-option${preset === "custom" ? " selected" : ""}`}
                onClick={() => setPreset("custom")}
                type="button"
              >
                <strong>自定义</strong>
                <span>最多 90 天</span>
              </button>
            </div>
            {preset === "custom" ? (
              <div className="catch-up-custom-range">
                <label>
                  <span>开始日期</span>
                  <Input max={yesterday} onChange={(event) => setFromDate(event.target.value)} type="date" value={fromDate} />
                </label>
                <label>
                  <span>结束日期</span>
                  <Input max={yesterday} onChange={(event) => setToDate(event.target.value)} type="date" value={toDate} />
                </label>
              </div>
            ) : null}
          </section>
        ) : null}

        {step === "chats" ? (
          <section className="catch-up-step">
            <div className="catch-up-step-heading">
              <span>第 2 步，共 2 步</span>
              <h3>选择群组</h3>
              <p>{language === "en"
                ? `${fromDate} to ${toDate} · ${selectedChatIds.size} chats selected.`
                : `${fromDate} 至 ${toDate}，已选择 ${selectedChatIds.size} 个群组。`}</p>
            </div>
            <div className="catch-up-selection-tools">
              <button className="text-link-button" onClick={() => setSelectedChatIds(new Set(chats.map((chat) => chat.id)))} type="button">
                全选
              </button>
              <button className="text-link-button" onClick={() => setSelectedChatIds(new Set())} type="button">
                清空
              </button>
            </div>
            <div className="catch-up-chat-list">
              {chats.map((chat) => (
                <label className="catch-up-chat-option" key={chat.id}>
                  <input checked={selectedChatIds.has(chat.id)} onChange={() => toggleChat(chat.id)} type="checkbox" />
                  <span>{chat.title}</span>
                </label>
              ))}
            </div>
            <div className={`catch-up-delivery-option${botReady ? "" : " disabled"}`}>
              <label>
                <input
                  checked={botReady && sendToTelegram}
                  disabled={!botReady}
                  onChange={(event) => setSendToTelegram(event.target.checked)}
                  type="checkbox"
                />
                <span>将结果发送到我的 Telegram</span>
              </label>
              {!botReady ? <span className="catch-up-tooltip" role="tooltip">此功能要求先配置 Telegram Bot</span> : null}
            </div>
          </section>
        ) : null}

        {step === "generating" ? (
          <section aria-live="polite" className="catch-up-generating" role="status">
            <span aria-hidden="true" className="catch-up-spinner" />
            <h3>正在生成快速回顾…</h3>
            <p>{language === "en"
              ? `The model is reading ${activeTask?.sourceSummaryCount ?? 0} daily summaries. You can close this dialog while the task continues in the background.`
              : `模型正在阅读 ${activeTask?.sourceSummaryCount ?? 0} 份每日摘要。你可以关闭弹窗，任务会在后台继续。`}</p>
          </section>
        ) : null}
      </div>
    </Modal>
  );
}

/** 使用指定时区的自然日计算最近若干个已结束日期。 */
function dateRangeForDays(days: number, timezone: string) {
  const parts = new Intl.DateTimeFormat("en", {
    timeZone: timezone || "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const values = new Map(parts.map((part) => [part.type, part.value]));
  const today = `${values.get("year")}-${values.get("month")}-${values.get("day")}`;
  const todayUTC = new Date(`${today}T12:00:00Z`);
  const end = new Date(todayUTC);
  end.setUTCDate(end.getUTCDate() - 1);
  const start = new Date(end);
  start.setUTCDate(start.getUTCDate() - days + 1);
  return { fromDate: formatUTCDate(start), toDate: formatUTCDate(end) };
}

function formatUTCDate(date: Date) {
  return date.toISOString().slice(0, 10);
}

function formatRange(range: { fromDate: string; toDate: string }) {
  return `${range.fromDate.slice(5)} – ${range.toDate.slice(5)}`;
}

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
