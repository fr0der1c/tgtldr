"use client";

import { useCallback, useEffect, useState } from "react";
import { SummaryDrawerPair } from "@/components/summary-drawer-pair";
import { api } from "@/lib/api";
import { Chat, Summary } from "@/lib/types";

export function ChatDaySummaryDrawer({
  chatId,
  chatTitle,
  date,
  onClose,
  open,
}: {
  chatId: number;
  chatTitle: string;
  date: string;
  onClose: () => void;
  open: boolean;
}) {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [chat, setChat] = useState<Chat | null>(null);
  const [botReady, setBotReady] = useState(false);
  const [summaryRetryLimit, setSummaryRetryLimit] = useState(2);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const loadSummary = useCallback(async () => {
    const response = await api.listSummaries({
      chatId: String(chatId), dateFrom: date, dateTo: date, page: 1, pageSize: 1,
    });
    setSummary(response.items[0] || null);
  }, [chatId, date]);

  useEffect(() => {
    if (!open || !date) return;
    let cancelled = false;
    setLoading(true);
    setError("");
    setSummary(null);
    void Promise.all([api.listChats(), api.settings(), loadSummary()]).then(([chats, settings]) => {
      if (cancelled) return;
      setChat(chats.find((item) => item.id === chatId) || null);
      setBotReady(
        settings.botEnabled
        && Boolean(settings.botToken?.trim())
        && Boolean(settings.botTargetChatId?.trim()),
      );
      setSummaryRetryLimit(settings.summaryRetryLimit ?? 2);
    }).catch((reason) => {
      if (!cancelled) setError(asMessage(reason));
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [chatId, date, loadSummary, open]);

  const emptyTitle = error ? "当天摘要加载失败" : "当天还没有摘要";
  const emptyDescription = error
    || "这个日期还没有生成摘要，可以前往摘要页手动生成。";

  return (
    <SummaryDrawerPair
      botReady={botReady}
      chatTitle={summary ? chat?.title || chatTitle : chatTitle}
      emptyDescription={emptyDescription}
      emptyTitle={emptyTitle}
      loading={loading}
      onClose={onClose}
      onRefresh={loadSummary}
      open={open}
      selectedChat={chat}
      selectedSummary={summary}
      summaryRetryLimit={summaryRetryLimit}
    />
  );
}

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : "服务器返回错误";
}
