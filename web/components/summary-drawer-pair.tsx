"use client";

import { useEffect, useState } from "react";
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

  useEffect(() => {
    if (!open) setContextOpen(false);
  }, [open]);

  useEffect(() => {
    if (!contextOpen || !selectedSummary) {
      setContextPreview(null);
      return;
    }
    let cancelled = false;
    setContextLoading(true);
    void api.summaryContextPreview(selectedSummary.id).then((preview) => {
      if (!cancelled) setContextPreview(preview);
    }).catch((error) => {
      if (cancelled) return;
      setContextPreview(null);
      toast.showError(asMessage(error));
    }).finally(() => {
      if (!cancelled) setContextLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [contextOpen, selectedSummary, toast]);

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
