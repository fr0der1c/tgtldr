"use client";

import { useEffect, useState } from "react";
import { api, APIError } from "@/lib/api";
import { ChatMessageSearchItem, ChatMessageSearchResponse } from "@/lib/types";
import { TextHighlight } from "@/components/text-highlight";
import { Button } from "@/components/ui";
import { useI18n } from "@/lib/i18n";

export function ChatMessageSearch({
  chatId,
  page,
  query,
  timezone,
  filters,
  onPageChange,
  onSelect,
}: {
  chatId: number;
  page: number;
  query: string;
  timezone: string;
  filters: boolean;
  onPageChange: (page: number) => void;
  onSelect: (item: ChatMessageSearchItem) => void;
}) {
  const { language } = useI18n();
  const [response, setResponse] = useState<ChatMessageSearchResponse | null>(null);
  const [error, setError] = useState("");
  const terms = query.split(/\s+/).filter(Boolean);

  useEffect(() => {
    const controller = new AbortController();
    setResponse(null);
    setError("");
    void api.searchChatMessages(chatId, {
      query, page, pageSize: 50, filters, signal: controller.signal,
    }).then(setResponse).catch((reason) => {
      if (!isAbortError(reason)) setError(asMessage(reason));
    });
    return () => controller.abort();
  }, [chatId, filters, page, query]);

  if (error) {
    return <div className="chat-search-state"><strong>搜索失败</strong><span>{error}</span></div>;
  }
  if (!response) {
    return <SearchSkeleton />;
  }
  if (response.total === 0) {
    return (
      <div className="chat-search-state">
        <strong>没有找到相关聊天记录</strong>
        <span>尝试减少关键词，或检查发言人名称和拼写。</span>
      </div>
    );
  }

  const totalPages = Math.max(1, Math.ceil(response.total / response.pageSize));
  return (
    <div className="chat-search-results">
      <div className="chat-search-summary">
        <strong>搜索结果</strong>
        <span>共 {response.total} 条</span>
      </div>
      <div className="chat-search-result-list">
        {response.items.map((item) => (
          <button className="chat-search-result" key={item.id} onClick={() => onSelect(item)} type="button">
            <div className="chat-search-result-meta">
              <strong>{item.senderName || "未知发言人"}</strong>
              {item.senderUsername ? <span>@{item.senderUsername}</span> : null}
              <time>{formatResultTime(item.messageTime, item.localDate, timezone, language)}</time>
            </div>
            <p><TextHighlight terms={terms} text={item.matchSnippet || "非文本消息，无文字说明"} /></p>
            <span className="chat-search-result-action">查看上下文</span>
          </button>
        ))}
      </div>
      <div className="chat-search-pagination">
        <Button disabled={response.page <= 1} onClick={() => onPageChange(response.page - 1)} type="button" variant="secondary">上一页</Button>
        <span>第 {response.page} 页，共 {totalPages} 页</span>
        <Button disabled={response.page >= totalPages} onClick={() => onPageChange(response.page + 1)} type="button" variant="secondary">下一页</Button>
      </div>
    </div>
  );
}

function SearchSkeleton() {
  return <div aria-label="正在搜索聊天记录" className="chat-search-skeleton">{Array.from({ length: 5 }, (_, index) => <span key={index} />)}</div>;
}

function formatResultTime(value: string, date: string, timezone: string, language: string) {
  const time = new Intl.DateTimeFormat(language, { hour: "2-digit", minute: "2-digit", hour12: false, timeZone: timezone }).format(new Date(value));
  return `${date} ${time}`;
}

function isAbortError(reason: unknown) {
  return reason instanceof DOMException && reason.name === "AbortError";
}

function asMessage(reason: unknown) {
  if (reason instanceof APIError || reason instanceof Error) return reason.message;
  return "服务器返回错误";
}
