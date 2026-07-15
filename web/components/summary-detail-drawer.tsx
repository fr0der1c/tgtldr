"use client";

import { startTransition, useState } from "react";
import { Drawer } from "@/components/drawer";
import { EmptyState } from "@/components/dashboard-page";
import { SummaryMarkdown } from "@/components/summary-markdown";
import { StatusPill } from "@/components/ui";
import { Chat, Summary } from "@/lib/types";
import { deliveryState, statusText, statusTone } from "@/components/summaries-panel-sections";

export function SummaryDetailDrawer({
  active,
  botReady,
  chatTitle,
  emptyDescription = "从列表中选择一条摘要后，这里会展示完整正文。",
  emptyTitle = "没有可查看的摘要",
  loading = false,
  onClose,
  onOpenContext,
  onRerunSummary,
  onRetryDelivery,
  open,
  selectedChat,
  selectedSummary,
  summaryRetryLimit
}: {
  active: boolean;
  botReady: boolean;
  chatTitle: string;
  emptyDescription?: string;
  emptyTitle?: string;
  loading?: boolean;
  onClose: () => void;
  onOpenContext: () => void;
  onRerunSummary: (summary: Summary) => Promise<void>;
  onRetryDelivery: (summary: Summary) => Promise<void>;
  open: boolean;
  selectedChat: Chat | null;
  selectedSummary: Summary | null;
  summaryRetryLimit: number;
}) {
  const selectedDelivery = selectedSummary
    ? deliveryState(selectedSummary, selectedChat, botReady)
    : null;

  return (
    <Drawer active={active} onClose={onClose} open={open}>
      {loading ? (
        <p className="muted">正在读取当天摘要…</p>
      ) : !selectedSummary ? (
        <EmptyState
          description={emptyDescription}
          title={emptyTitle}
        />
      ) : (
        <div className="summary-detail-stack">
          <div className="summary-detail-header">
            <h2>
              {chatTitle} · {selectedSummary.summaryDate}
            </h2>
          </div>
          <div className="summary-status-actions">
            <StatusPill tone={statusTone(selectedSummary.status)}>
              {statusText(selectedSummary.status)}
            </StatusPill>
            <StatusPill
              className={selectedDelivery?.detail ? "status-pill-hoverable" : undefined}
              title={selectedDelivery?.detail}
              tone={selectedDelivery?.tone ?? "neutral"}
            >
              {selectedDelivery?.label ?? "不发送"}
            </StatusPill>
            {selectedDelivery?.retryable ? (
              <button
                className="text-link-button summary-delivery-link"
                onClick={() => startTransition(() => void onRetryDelivery(selectedSummary))}
                type="button"
              >
                通过 Bot 发送
              </button>
            ) : null}
          </div>
          <div className="summary-detail-meta">
            <p>
              {selectedSummary.model || "未记录模型"} · 消息 {selectedSummary.sourceMessageCount} 条 · 分块{" "}
              {selectedSummary.chunkCount}
            </p>
            <div className="summary-detail-meta-actions">
              <button className="text-link-button" onClick={onOpenContext} type="button">
                查看原始 prompt
              </button>
              <button
                className="text-link-button"
                onClick={() => startTransition(() => void onRerunSummary(selectedSummary))}
                type="button"
              >
                重新生成
              </button>
            </div>
          </div>
          <SummaryContent summary={selectedSummary} summaryRetryLimit={summaryRetryLimit} />
        </div>
      )}
    </Drawer>
  );
}

function SummaryContent({
  summary,
  summaryRetryLimit,
}: {
  summary: Summary;
  summaryRetryLimit: number;
}) {
  if (summary.status === "failed") {
    return (
      <div className="summary-context-stack">
        <RetryStatus summary={summary} summaryRetryLimit={summaryRetryLimit} />
        <div>
          <p className="muted">服务器返回错误</p>
          <pre className="summary-context-block">{summary.errorMessage || ""}</pre>
        </div>
        {summary.errorContext ? (
          <CopyableContextBlock title="OpenAI 请求参数" value={summary.errorContext} />
        ) : null}
        {summary.errorSystemPrompt ? (
          <CopyableContextBlock title="System prompt" value={summary.errorSystemPrompt} />
        ) : null}
        {summary.errorUserPrompt ? (
          <CopyableContextBlock title="User prompt" value={summary.errorUserPrompt} />
        ) : null}
      </div>
    );
  }

  if (!summary.content) {
    return (
      <EmptyState
        description="这条摘要还没有正文，请稍后重试或重新生成。"
        title="还没有摘要内容"
      />
    );
  }

  return (
    <div className="summary-detail-content">
      <SummaryMarkdown content={summary.content} />
    </div>
  );
}

function RetryStatus({
  summary,
  summaryRetryLimit,
}: {
  summary: Summary;
  summaryRetryLimit: number;
}) {
  const retryCount = summary.retryCount || 0;
  const limitLabel =
    summaryRetryLimit > 0 ? `${retryCount} / ${summaryRetryLimit}` : `${retryCount} / 0`;
  let detail = "未安排自动重试。";
  if (summary.nextRetryAt) {
    detail = `下次自动重试：${formatRetryTime(summary.nextRetryAt)}`;
  } else if (summaryRetryLimit > 0 && retryCount >= summaryRetryLimit) {
    detail = "已达到自动重试上限。";
  } else if (summaryRetryLimit === 0) {
    detail = "自动重试已关闭。";
  }

  return (
    <div className="summary-retry-status">
      <p>已自动重试 {limitLabel}</p>
      <span>{detail}</span>
    </div>
  );
}

function formatRetryTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function CopyableContextBlock({ title, value }: { title: string; value: string }) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");

  async function handleCopy() {
    try {
      await copyTextToClipboard(value);
      setCopyState("copied");
    } catch {
      setCopyState("failed");
    }
    window.setTimeout(() => setCopyState("idle"), 1500);
  }

  return (
    <div className="summary-error-context-section">
      <div className="summary-error-context-head">
        <p className="muted">{title}</p>
        <button className="text-link-button" onClick={handleCopy} type="button">
          {copyButtonLabel(copyState)}
        </button>
      </div>
      <pre className="summary-context-block summary-error-context-block">{value}</pre>
    </div>
  );
}

async function copyTextToClipboard(value: string) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();

  try {
    if (!document.execCommand("copy")) {
      throw new Error("copy command failed");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

function copyButtonLabel(state: "idle" | "copied" | "failed") {
  if (state === "copied") {
    return "已复制";
  }
  if (state === "failed") {
    return "复制失败";
  }
  return "复制";
}
