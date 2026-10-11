"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { dateRangeForDays } from "@/components/catch-up-wizard";
import { useToast } from "@/components/toast";
import { Button, Field, Input } from "@/components/ui";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import type { Chat, SummaryBatchItem } from "@/lib/types";

const pollIntervalMs = 5000;
// 与后端一致：日期范围最多 90 天，单批最多 200 条。
const maxDays = 90;
const maxItems = 200;
// 记在模块级而不是组件内：折叠“手动生成”会卸载本组件，重新展开时仍要知道期间是否有批次结束。
let batchWasRunning = false;

/** SummaryBatchRun 为多个群组按日期范围排队生成摘要，并显示后台批次的逐项结果。 */
export function SummaryBatchRun({
  chats,
  onChanged,
  timezone,
}: {
  chats: Chat[];
  onChanged: () => void;
  timezone: string;
}) {
  const { language } = useI18n();
  const toast = useToast();
  const [selectedChatIds, setSelectedChatIds] = useState<Set<number>>(new Set());
  const [fromDate, setFromDate] = useState(() => dateRangeForDays(7, timezone).fromDate);
  const [toDate, setToDate] = useState(() => dateRangeForDays(7, timezone).toDate);
  const [overwrite, setOverwrite] = useState(false);
  const [items, setItems] = useState<SummaryBatchItem[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const yesterday = dateRangeForDays(1, timezone).toDate;
  const running = items.some((item) => item.status === "queued");
  const onChangedRef = useRef(onChanged);
  // 每次读取或提交都递增；较早发出的读取晚于提交返回时丢弃，避免旧列表覆盖新批次。
  const requestRef = useRef(0);

  const refresh = useCallback(async () => {
    const request = ++requestRef.current;
    try {
      const response = await api.summaryBatch();
      if (request !== requestRef.current) {
        return;
      }
      setItems(response.items);
      setLoaded(true);
    } catch {
      // 进度读取失败时保留上次结果，下一轮继续尝试。
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    onChangedRef.current = onChanged;
  }, [onChanged]);

  useEffect(() => {
    if (!running) {
      return;
    }
    const timer = window.setInterval(() => void refresh(), pollIntervalMs);
    return () => window.clearInterval(timer);
  }, [refresh, running]);

  // 批次结束时刷新摘要列表和统计，让新结果立即可见；首次读取完成前不改变记录的状态。
  useEffect(() => {
    if (!loaded) {
      return;
    }
    if (batchWasRunning && !running) {
      onChangedRef.current();
    }
    batchWasRunning = running;
  }, [loaded, running]);

  function toggleChat(id: number) {
    setSelectedChatIds((current) => {
      const next = new Set(current);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
  }

  async function submit() {
    setSubmitting(true);
    try {
      requestRef.current++;
      const result = await api.startSummaryBatch({ chatIds: [...selectedChatIds], fromDate, toDate, overwrite });
      requestRef.current++;
      setItems(result.items);
      setLoaded(true);
      toast.showSuccess(batchQueuedMessage(result.queued, result.skipped, language));
    } catch (err) {
      toast.showError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  }

  const days = countDays(fromDate, toDate);
  const overLimit = days > maxDays || selectedChatIds.size * days > maxItems;
  const deliveringChats = chats.filter((chat) => selectedChatIds.has(chat.id) && chat.deliveryMode === "bot").length;

  return (
    <div className="summary-batch">
      <div className="summary-manual-head">
        <strong>批量生成</strong>
        <p>为多个群组按日期范围排队生成，后台每次处理两条，可以离开页面。</p>
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
            <span data-i18n-skip="true">{chat.title}</span>
          </label>
        ))}
      </div>
      <div className="form-grid">
        <Field label="开始日期">
          <Input max={toDate || yesterday} onChange={(event) => setFromDate(event.target.value)} type="date" value={fromDate} />
        </Field>
        <Field label="结束日期">
          <Input max={yesterday} onChange={(event) => setToDate(event.target.value)} type="date" value={toDate} />
        </Field>
      </div>
      <label className="summary-batch-option">
        <input checked={overwrite} onChange={(event) => setOverwrite(event.target.checked)} type="checkbox" />
        <span>重新生成已成功的摘要</span>
      </label>
      {selectedChatIds.size > 0 && days > 0 && !overLimit ? (
        <p className="summary-batch-note">{batchPlanMessage(selectedChatIds.size, days, overwrite, deliveringChats, language)}</p>
      ) : null}
      {overLimit ? (
        <p className="summary-batch-limit" role="alert">
          {language === "en"
            ? `A batch covers at most ${maxDays} days and ${maxItems} summaries. Select fewer chats or a shorter range.`
            : `单批最多 ${maxDays} 天、${maxItems} 条摘要，请减少群组或缩短日期范围。`}
        </p>
      ) : null}
      <div className="summary-manual-actions">
        <Button disabled={submitting || running || overLimit || selectedChatIds.size === 0 || days <= 0} onClick={() => void submit()} type="button">
          {running ? "批量生成中…" : "批量生成"}
        </Button>
      </div>
      {items.length > 0 ? <SummaryBatchProgress items={items} /> : null}
    </div>
  );
}

function SummaryBatchProgress({ items }: { items: SummaryBatchItem[] }) {
  const { language } = useI18n();
  const failed = items.filter((item) => item.status === "failed");
  const skipped = items.filter((item) => item.status === "skipped").length;
  const finished = items.filter((item) => item.status !== "queued").length;
  return (
    <div aria-live="polite" className="summary-batch-progress">
      <p className="summary-batch-note">
        {language === "en"
          ? `Last batch: ${finished} of ${items.length} finished, ${failed.length} failed, ${skipped} skipped because they already succeeded or AI summaries were turned off.`
          : `最近一批：已完成 ${finished}/${items.length}，失败 ${failed.length} 条，跳过 ${skipped} 条（已生成成功或已关闭 AI 总结）。`}
      </p>
      {failed.length > 0 ? (
        <ul className="summary-batch-failures">
          {failed.map((item) => (
            <li key={`${item.chatId}-${item.date}`}>
              <strong>
                <span data-i18n-skip="true">{item.chatTitle}</span> · {item.date}
              </strong>
              <span data-i18n-skip="true">{item.error}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function countDays(fromDate: string, toDate: string) {
  const from = Date.parse(`${fromDate}T00:00:00Z`);
  const to = Date.parse(`${toDate}T00:00:00Z`);
  if (Number.isNaN(from) || Number.isNaN(to) || to < from) {
    return 0;
  }
  return Math.round((to - from) / 86_400_000) + 1;
}

function batchPlanMessage(chats: number, days: number, overwrite: boolean, delivering: number, language: string) {
  const total = chats * days;
  const plan = language === "en"
    ? `${chats} chats × ${days} days = up to ${total} summaries${overwrite ? "" : ", skipping ones that already succeeded"}.`
    : `${chats} 个群组 × ${days} 天，最多 ${total} 条摘要${overwrite ? "" : "，已成功的会跳过"}。`;
  if (delivering === 0) {
    return plan;
  }
  return language === "en"
    ? `${plan} ${delivering} of the selected chats take part in Telegram delivery; each summary follows the delivery settings, the same as Generate now.`
    : `${plan}其中 ${delivering} 个群组参与 Telegram 推送，每条摘要会和“立即生成”一样按推送设置处理。`;
}

function batchQueuedMessage(queued: number, skipped: number, language: string) {
  if (language === "en") {
    return queued === 0
      ? `Nothing to generate; ${skipped} summaries already succeeded.`
      : `Queued ${queued} summaries${skipped > 0 ? `, skipped ${skipped} that already succeeded` : ""}.`;
  }
  return queued === 0
    ? `没有需要生成的摘要，${skipped} 条已生成成功。`
    : `已排队 ${queued} 条摘要${skipped > 0 ? `，跳过 ${skipped} 条已成功的摘要` : ""}。`;
}
