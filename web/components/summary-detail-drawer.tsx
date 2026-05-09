"use client";

import { startTransition, useState } from "react";
import { Drawer } from "@/components/drawer";
import { EmptyState } from "@/components/dashboard-page";
import { SummaryMarkdown } from "@/components/summary-markdown";
import { StatusPill } from "@/components/ui";
import { Chat, Summary } from "@/lib/types";
import { deliveryState, statusText, statusTone } from "@/components/summaries-panel-sections";

export function SummaryDetailDrawer({
  botReady,
  chatTitle,
  onClose,
  onOpenContext,
  onRerunSummary,
  onRetryDelivery,
  open,
  selectedChat,
  selectedSummary
}: {
  botReady: boolean;
  chatTitle: string;
  onClose: () => void;
  onOpenContext: () => void;
  onRerunSummary: (summary: Summary) => Promise<void>;
  onRetryDelivery: (summary: Summary) => Promise<void>;
  open: boolean;
  selectedChat: Chat | null;
  selectedSummary: Summary | null;
}) {
  const selectedDelivery = selectedSummary
    ? deliveryState(selectedSummary, selectedChat, botReady)
    : null;

  return (
    <Drawer onClose={onClose} open={open}>
      {!selectedSummary ? (
        <EmptyState
          description="从列表中选择一条摘要后，这里会展示完整正文。"
          title="没有可查看的摘要"
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
          <SummaryContent summary={selectedSummary} />
        </div>
      )}
    </Drawer>
  );
}

function SummaryContent({ summary }: { summary: Summary }) {
  if (summary.status === "failed") {
    return (
      <div className="summary-context-stack">
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

function CopyableContextBlock({ title, value }: { title: string; value: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="summary-error-context-section">
      <div className="summary-error-context-head">
        <p className="muted">{title}</p>
        <button className="text-link-button" onClick={handleCopy} type="button">
          {copied ? "已复制" : "复制"}
        </button>
      </div>
      <pre className="summary-context-block summary-error-context-block">{value}</pre>
    </div>
  );
}
