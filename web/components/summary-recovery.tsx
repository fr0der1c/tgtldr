"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useToast } from "@/components/toast";

import type { RecoveryItem, FailureNotice } from "@/lib/types";

const notificationErrors: Record<string, string> = {
  request: "Telegram 请求被拒绝，请检查目标聊天配置。",
  auth: "Telegram Bot 凭据无效。",
  permission: "Telegram Bot 没有向目标聊天发送消息的权限。",
  rate_limit: "Telegram 通知发送受到限流。",
  upstream: "Telegram 服务暂时不可用。",
  timeout: "Telegram 通知发送超时。",
};

/** 展示全量失败任务恢复入口，并持续读取后台进度及通知投递错误。 */
export function SummaryRecovery({ onChanged }: { onChanged: () => void }) {
  const [items, setItems] = useState<RecoveryItem[]>([]);
  const [notices, setNotices] = useState<FailureNotice[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [ready, setReady] = useState(false);
  const toast = useToast();
  const changedRef = useRef(onChanged);
  changedRef.current = onChanged;
  const queued = items.filter(item => item.status === "queued").length;
  const failed = items.filter(item => item.status === "failed");

  // 挂载后读取持久化状态，关闭网页不会中断后台任务。
  useEffect(() => {
    let disposed = false;
    let previousProgress = "";
    /** 读取批次变化后同步摘要列表，避免队列结束时统计停留在旧状态。 */
    async function refresh() {
      try {
        const [batch, notifications] = await Promise.all([api.summaryRecovery(), api.failureNotices()]);
        if (disposed) return;
        setItems(batch);
        setNotices(notifications);
        setReady(true);
        const progress = batch.map(item => `${item.kind}:${item.id}:${item.status}`).join(",");
        if (progress !== previousProgress || batch.some(item => item.status === "queued")) changedRef.current();
        previousProgress = progress;
      } catch (error) {
        if (!disposed) toast.showError(error instanceof Error ? error.message : String(error));
      }
    }
    void refresh();
    const timer = window.setInterval(() => void refresh(), 5000);
    return () => { disposed = true; window.clearInterval(timer); };
  }, []);

  /** 提交全量失败任务快照，并刷新摘要列表统计。 */
  async function retryAll() {
    setSubmitting(true);
    try {
      setItems(await api.startSummaryRecovery());
      toast.showSuccess("已提交失败任务重跑，完成后会补发失败的每日总览。");
      onChanged();
    } catch (error) {
      toast.showError(error instanceof Error ? error.message : String(error));
    } finally {
      setSubmitting(false);
    }
  }

  return <div className="summary-recovery" aria-label="失败任务恢复">
    <button className="summary-recovery-button" type="button"
      title="重跑全部失败摘要，保留成功结果；完成后重新生成并发送失败的每日总览。"
      disabled={!ready || submitting || queued > 0} onClick={() => void retryAll()}>
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
        <path d="M20 7v5h-5M4 17v-5h5" strokeLinecap="round" strokeLinejoin="round" />
        <path d="M6.1 7a7 7 0 0 1 11.7-1L20 9M4 15l2.2 3A7 7 0 0 0 17.9 17" strokeLinecap="round" />
      </svg>
      <span>{queued > 0 || submitting ? "重跑中" : "一键重跑"}</span>
    </button>
    {(items.length > 0 || notices.some(item => item.error)) && <details className="summary-recovery-details">
      <summary aria-label="重跑详情" title="重跑详情"><span aria-hidden="true">⋯</span></summary>
      <div className="summary-recovery-popover">
        {items.length > 0 && <div><span>已完成</span> {items.length - queued} / {items.length} · <span>失败</span> {failed.length}</div>}
        {failed.length > 0 && <div>
          <div className="summary-recovery-heading">重跑失败详情</div>
          {failed.map(item => <div key={`${item.kind}:${item.id}`}>{item.date} · {item.kind === "summary" ? "群组摘要" : "每日总览"} #{item.id} · {item.error}</div>)}
        </div>}
        {notices.some(item => item.error) && <div>
          <div className="summary-recovery-heading">失败通知发送异常</div>
          {notices.filter(item => item.error).map(item => <div key={item.date}>{item.date} · <span>发送尝试次数</span> {item.attempts} / 4 · <span>{notificationErrors[item.error] ?? "Telegram 通知发送失败"}</span></div>)}
        </div>}
      </div>
    </details>}
    {queued > 0 && <span className="summary-recovery-progress" role="status">{items.length - queued} / {items.length}</span>}
  </div>;
}
