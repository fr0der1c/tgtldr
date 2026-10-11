"use client";

import { useEffect, useRef, useState } from "react";
import { SummaryContextDrawer } from "@/components/summary-context-drawer";
import { SummaryDetailDrawer } from "@/components/summary-detail-drawer";
import { useToast } from "@/components/toast";
import { api } from "@/lib/api";
import { Chat, Summary, SummaryContextPreview } from "@/lib/types";

export function SummaryDrawerPair({
  botReady,
  chatTitle,
  emptyDescription,
  emptyTitle,
  loading = false,
  onClose,
  onRefresh,
  open,
  selectedChat,
  selectedSummary,
  summaryRetryLimit,
}: {
  botReady: boolean;
  chatTitle: string;
  emptyDescription?: string;
  emptyTitle?: string;
  loading?: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  open: boolean;
  selectedChat: Chat | null;
  selectedSummary: Summary | null;
  summaryRetryLimit: number;
}) {
  const [contextOpen, setContextOpen] = useState(false);
  const [contextPreview, setContextPreview] = useState<SummaryContextPreview | null>(null);
  const [contextLoading, setContextLoading] = useState(false);
  const toast = useToast();
  const showErrorRef = useRef(toast.showError);
  // 摘要列表处理中会每 3 秒轮询刷新，只按摘要 ID 加载原始 Prompt，避免侧栏反复重载。
  const selectedSummaryId = selectedSummary?.id ?? null;

  useEffect(() => {
    showErrorRef.current = toast.showError;
  }, [toast.showError]);

  useEffect(() => {
    if (!open) setContextOpen(false);
  }, [open]);

  useEffect(() => {
    if (!contextOpen || selectedSummaryId === null) {
      setContextPreview(null);
      return;
    }
    let cancelled = false;
    setContextLoading(true);
    void api.summaryContextPreview(selectedSummaryId).then((preview) => {
      if (!cancelled) setContextPreview(preview);
    }).catch((error) => {
      if (cancelled) return;
      setContextPreview(null);
      showErrorRef.current(asMessage(error));
    }).finally(() => {
      if (!cancelled) setContextLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [contextOpen, selectedSummaryId]);

  async function retryDelivery(summary: Summary) {
    try {
      await api.retrySummaryDelivery(summary.id);
      toast.showSuccess("已提交通过 Bot 发送。");
      await onRefresh();
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  async function rerunSummary(summary: Summary) {
    try {
      await api.runSummary(summary.chatId, summary.summaryDate);
      toast.showSuccess("已提交重新生成。");
      await onRefresh();
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  return (
    <>
      <SummaryDetailDrawer
        active={!contextOpen}
        botReady={botReady}
        chatTitle={chatTitle}
        emptyDescription={emptyDescription}
        emptyTitle={emptyTitle}
        loading={loading}
        onClose={onClose}
        onOpenContext={() => setContextOpen(true)}
        onRerunSummary={rerunSummary}
        onRetryDelivery={retryDelivery}
        open={open}
        selectedChat={selectedChat}
        selectedSummary={selectedSummary}
        summaryRetryLimit={summaryRetryLimit}
      />
      <SummaryContextDrawer
        loading={contextLoading}
        onClose={() => setContextOpen(false)}
        open={contextOpen}
        preview={contextPreview}
      />
    </>
  );
}

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
