"use client";

import { startTransition, useEffect, useRef, useState } from "react";
import { Drawer } from "@/components/drawer";
import { EmptyState } from "@/components/dashboard-page";
import { SummaryMarkdown } from "@/components/summary-markdown";
import { Button, StatusPill, Textarea } from "@/components/ui";
import { Chat, Summary } from "@/lib/types";
import { deliveryState, statusText, statusTone } from "@/components/summaries-panel-sections";
import { api } from "@/lib/api";

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

  const canAsk = Boolean(
    selectedSummary?.status === "succeeded" &&
    selectedSummary.content &&
    (selectedSummary.sourceMessageCount ?? 0) > 0
  );

  return (
    <Drawer
      footer={canAsk && selectedSummary ? <FollowUpBox summaryId={selectedSummary.id} /> : undefined}
      onClose={onClose}
      open={open}
    >
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
    return <pre className="summary-context-block">{summary.errorMessage || ""}</pre>;
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

type QAPair = { question: string; answer: string | null };

const STORAGE_KEY = (id: number) => `followup_${id}`;

function FollowUpBox({ summaryId }: { summaryId: number }) {
  const [pairs, setPairs] = useState<QAPair[]>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY(summaryId));
      return saved ? (JSON.parse(saved) as QAPair[]) : [];
    } catch {
      return [];
    }
  });
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const threadRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    try {
      const toSave = pairs.filter((p) => p.answer !== null);
      localStorage.setItem(STORAGE_KEY(summaryId), JSON.stringify(toSave));
    } catch {}
  }, [pairs, summaryId]);

  useEffect(() => {
    if (threadRef.current) {
      threadRef.current.scrollTop = threadRef.current.scrollHeight;
    }
  }, [pairs]);

  async function submit() {
    const q = input.trim();
    if (!q || loading) return;
    setInput("");
    setLoading(true);
    setError(null);

    const history = pairs
      .filter((p) => p.answer !== null)
      .map((p) => ({ question: p.question, answer: p.answer as string }));

    setPairs((prev) => [...prev, { question: q, answer: null }]);

    try {
      const { answer } = await api.askSummaryFollowUp(summaryId, q, history);
      setPairs((prev) =>
        prev.map((p, i) => (i === prev.length - 1 ? { ...p, answer } : p))
      );
    } catch (e) {
      setPairs((prev) => prev.slice(0, -1));
      setError(e instanceof Error ? e.message : "请求失败，请稍后重试。");
    } finally {
      setLoading(false);
      inputRef.current?.focus();
    }
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void submit();
    }
  }

  return (
    <div className="followup-box">
      <p className="followup-label">追问当天内容</p>
      {pairs.length > 0 ? (
        <div className="followup-thread" ref={threadRef}>
          {pairs.map((pair, i) => (
            <div className="followup-pair" key={i}>
              <div className="followup-question">{pair.question}</div>
              <div className="followup-answer">
                {pair.answer === null ? (
                  <span className="followup-thinking">思考中…</span>
                ) : (
                  <SummaryMarkdown content={pair.answer} />
                )}
              </div>
            </div>
          ))}
        </div>
      ) : null}
      {error ? <p className="followup-error">{error}</p> : null}
      <div className="followup-input-row">
        <Textarea
          disabled={loading}
          onKeyDown={handleKeyDown}
          onChange={(e) => setInput(e.target.value)}
          placeholder="输入问题，按 Enter 发送（Shift+Enter 换行）"
          ref={inputRef}
          rows={2}
          value={input}
        />
        <Button
          disabled={!input.trim() || loading}
          onClick={() => void submit()}
          type="button"
        >
          发送
        </Button>
      </div>
    </div>
  );
}
