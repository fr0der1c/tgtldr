"use client";

import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import {
  UIEvent,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { api } from "@/lib/api";
import { APIError } from "@/lib/api";
import { ChatMessage, ChatMessageListResponse } from "@/lib/types";
import { useI18n } from "@/lib/i18n";
import { ChatMessageItem } from "@/components/chat-message-item";
import { ChatMessageSearch } from "@/components/chat-message-search";
import { ChatHistorySearchForm } from "@/components/chat-history-search-form";
import { ChatDaySummaryDrawer } from "@/components/chat-day-summary-drawer";
import { ChatHistoryToolbar } from "@/components/chat-history-toolbar";
import { DashboardPage, EmptyState, Surface } from "@/components/dashboard-page";
import { Button } from "@/components/ui";

const initialMessageLimit = 2000;
const earlierMessageLimit = 500;

export function ChatMessagesPanel({
  chatId,
}: {
  chatId: number;
}) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const selectedDate = searchParams.get("date") || "";
  const query = searchParams.get("q") || "";
  const searchPage = positiveSearchParam(searchParams.get("searchPage"), 1);
  const focusedMessageId = positiveSearchParam(searchParams.get("messageId"), 0);
  const filtersApplied = searchParams.get("filters") === "1";
  const summaryDate = searchParams.get("summaryDate") || "";
  const { language } = useI18n();
  const [metadata, setMetadata] = useState<ChatMessageListResponse | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [error, setError] = useState("");
  const [searchOpen, setSearchOpen] = useState(Boolean(query));
  const metadataRef = useRef<ChatMessageListResponse | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const pendingScrollHeightRef = useRef<number | null>(null);
  const scrollToBottomRef = useRef(false);
  const scrollToFocusRef = useRef(0);
  const earlierAbortRef = useRef<AbortController | null>(null);
  const resolvedDateRef = useRef("");

  useEffect(() => {
    if (
      query && focusedMessageId === 0 && metadataRef.current
      && metadataRef.current.filtersApplied === filtersApplied
    ) {
      setLoading(false);
      return;
    }
    if (selectedDate && resolvedDateRef.current === selectedDate) {
      resolvedDateRef.current = "";
      return;
    }
    const controller = new AbortController();
    earlierAbortRef.current?.abort();
    setLoading(true);
    setLoadingEarlier(false);
    setError("");
    setMessages([]);
    pendingScrollHeightRef.current = null;

    void api.listChatMessages(chatId, {
      date: selectedDate || undefined,
      limit: initialMessageLimit,
      focusMessageId: focusedMessageId || undefined,
      filters: filtersApplied,
      signal: controller.signal,
    }).then((response) => {
      metadataRef.current = response;
      setMetadata(response);
      setMessages(response.messages);
      if (response.focusedMessageId) {
        scrollToFocusRef.current = response.focusedMessageId;
      } else {
        scrollToBottomRef.current = true;
      }
      if (response.date !== selectedDate) {
        const params = new URLSearchParams(window.location.search);
        params.set("date", response.date);
        resolvedDateRef.current = response.date;
        window.history.replaceState(null, "", `${pathname}?${params.toString()}`);
      }
    }).catch((reason) => {
      if (!isAbortError(reason)) setError(asMessage(reason));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });

    return () => controller.abort();
  }, [chatId, filtersApplied, focusedMessageId, pathname, query, selectedDate]);

  useEffect(() => {
    if (query) setSearchOpen(true);
  }, [query]);

  useLayoutEffect(() => {
    const container = scrollRef.current;
    if (!container) return;
    if (scrollToFocusRef.current > 0) {
      const target = container.querySelector<HTMLElement>(`[data-message-id="${scrollToFocusRef.current}"]`);
      target?.scrollIntoView({ block: "center" });
      scrollToFocusRef.current = 0;
      return;
    }
    if (scrollToBottomRef.current) {
      container.scrollTop = container.scrollHeight;
      scrollToBottomRef.current = false;
      return;
    }
    if (pendingScrollHeightRef.current === null) return;
    container.scrollTop += container.scrollHeight - pendingScrollHeightRef.current;
    pendingScrollHeightRef.current = null;
  }, [messages]);

  async function loadEarlier() {
    if (!metadata?.hasMoreBefore || !metadata.beforeCursor || loadingEarlier) return;
    const controller = new AbortController();
    earlierAbortRef.current = controller;
    const requestedDate = metadata.date;
    setLoadingEarlier(true);
    setError("");
    if (scrollRef.current) {
      pendingScrollHeightRef.current = scrollRef.current.scrollHeight;
    }
    try {
      const response = await api.listChatMessages(chatId, {
        date: requestedDate,
        limit: earlierMessageLimit,
        before: metadata.beforeCursor,
        filters: filtersApplied,
        signal: controller.signal,
      });
      setMetadata(response);
      setMessages((current) => prependUnique(response.messages, current));
    } catch (reason) {
      pendingScrollHeightRef.current = null;
      if (!isAbortError(reason)) setError(asMessage(reason));
    } finally {
      if (!controller.signal.aborted) setLoadingEarlier(false);
    }
  }

  function navigateToDate(date: string) {
    if (loading || !date || date === metadata?.date) return;
    earlierAbortRef.current?.abort();
    const params = currentSearchParams();
    params.set("date", date);
    params.delete("messageId");
    params.delete("q");
    params.delete("searchPage");
    params.delete("summaryDate");
    pushSearchParams(pathname, params);
  }

  function navigateToSearch(query: string) {
    const params = currentSearchParams();
    params.delete("messageId");
    params.delete("searchPage");
    if (query) params.set("q", query);
    else params.delete("q");
    pushSearchParams(pathname, params);
  }

  function navigateToSearchPage(page: number) {
    const params = currentSearchParams();
    params.set("searchPage", String(page));
    pushSearchParams(pathname, params);
  }

  function changeFilters(enabled: boolean) {
    const params = currentSearchParams();
    params.delete("messageId");
    params.delete("searchPage");
    if (enabled) params.set("filters", "1");
    else params.delete("filters");
    pushSearchParams(pathname, params);
  }

  function openDaySummary() {
    const date = selectedDate || metadata?.date;
    if (!date) return;
    const params = currentSearchParams();
    params.set("summaryDate", date);
    pushSearchParams(pathname, params);
  }

  function closeDaySummary() {
    const params = currentSearchParams();
    params.delete("summaryDate");
    pushSearchParams(pathname, params);
  }

  function focusSearchResult(item: ChatMessage) {
    const params = currentSearchParams();
    const localDate = "localDate" in item ? String(item.localDate) : metadata?.date || "";
    params.set("date", localDate);
    params.set("messageId", String(item.id));
    pushSearchParams(pathname, params);
  }

  function leaveFocusedMessage(keepSearch: boolean) {
    const params = currentSearchParams();
    params.delete("messageId");
    if (!keepSearch) {
      params.delete("q");
      params.delete("searchPage");
    }
    pushSearchParams(pathname, params);
  }

  function handleScroll(event: UIEvent<HTMLDivElement>) {
    if (event.currentTarget.scrollTop <= 160) void loadEarlier();
  }

  const title = metadata?.chat.title || "聊天记录";
  return (
    <DashboardPage
      description="按日期查看群聊记录，向上滚动可加载当天更早的消息。"
      title={title}
    >
      <Surface
        actions={(
          <div className="chat-history-head-actions">
            <button
              aria-label={searchOpen ? "收起聊天记录搜索" : "展开聊天记录搜索"}
              aria-pressed={searchOpen}
              className={`chat-history-icon-button${searchOpen ? " active" : ""}`}
              onClick={() => {
                if (searchOpen && query) navigateToSearch("");
                setSearchOpen((open) => !open);
              }}
              title={searchOpen ? "收起搜索" : "搜索聊天记录"}
              type="button"
            >
              <SearchIcon />
            </button>
            <Link
              aria-label="查看摘要"
              className="chat-history-icon-button"
              href={`/dashboard/summaries?chatId=${encodeURIComponent(String(chatId))}`}
              title="查看摘要"
            >
              <SummaryIcon />
            </Link>
          </div>
        )}
        className="chat-history-surface"
        leading={(
          <Link
            aria-label="返回群组列表"
            className="chat-history-back-link"
            href="/dashboard/chats"
            title="返回群组列表"
          >
            <BackIcon />
          </Link>
        )}
        title="聊天记录"
      >
        {searchOpen ? (
          <ChatHistorySearchForm onQueryChange={navigateToSearch} query={query} />
        ) : null}

        {metadata ? (
          <ChatHistoryToolbar
            activity={metadata.messageActivity}
            busy={loading}
            currentDate={selectedDate || metadata.date}
            nextDate={metadata.nextDate}
            onDateChange={navigateToDate}
            filtersApplied={filtersApplied && metadata.hasMessageFilters}
            hasMessageFilters={metadata.hasMessageFilters}
            onFiltersChange={changeFilters}
            previousDate={metadata.previousDate}
            total={metadata.date === (selectedDate || metadata.date) ? metadata.total : undefined}
          />
        ) : null}

        {focusedMessageId > 0 && metadata ? (
          <div className="chat-focus-actions">
            <span>正在查看搜索结果上下文</span>
            <div>
              {query ? <Button onClick={() => leaveFocusedMessage(true)} type="button" variant="ghost">返回搜索结果</Button> : null}
              <Button onClick={() => leaveFocusedMessage(false)} type="button" variant="secondary">查看当天最新消息</Button>
            </div>
          </div>
        ) : null}

        {loading ? <ChatHistorySkeleton /> : null}
        {!loading && error && messages.length === 0 && !query ? (
          <EmptyState title="聊天记录加载失败" description={error} />
        ) : null}
        {!loading && metadata && metadata.total === 0 && !query ? (
          <EmptyState title="当天没有聊天记录" description="可以选择其他日期，或先为该群启用消息保存。" />
        ) : null}
        {metadata && query && focusedMessageId === 0 ? (
          <ChatMessageSearch
            chatId={chatId}
            onPageChange={navigateToSearchPage}
            onSelect={focusSearchResult}
            page={searchPage}
            query={query}
            filters={filtersApplied}
            timezone={metadata.timezone}
          />
        ) : null}
        {!loading && messages.length > 0 && metadata && (!query || focusedMessageId > 0) ? (
          <div
            aria-label={`${metadata.date} 聊天记录`}
            className="chat-message-scroll"
            onScroll={handleScroll}
            ref={scrollRef}
            tabIndex={0}
          >
            <div className="chat-message-load-earlier">
              {metadata.hasMoreBefore ? (
                <Button disabled={loadingEarlier} onClick={() => void loadEarlier()} type="button" variant="ghost">
                  {loadingEarlier ? "正在加载更早消息…" : "加载更早消息"}
                </Button>
              ) : (
                <span>
                  已加载当天全部 {metadata.total} 条消息，
                  <button
                    className="chat-day-summary-link"
                    onClick={openDaySummary}
                    type="button"
                  >
                    查看摘要
                  </button>
                </span>
              )}
              {error ? <span className="chat-message-inline-error">{error}</span> : null}
            </div>
            <div className="chat-message-list">
              {messages.map((message) => (
                <ChatMessageItem
                  highlighted={message.id === focusedMessageId}
                  key={message.id}
                  language={language}
                  message={message}
                  timezone={metadata.timezone}
                />
              ))}
            </div>
            <div aria-hidden="true" className="chat-message-end-marker" />
          </div>
        ) : null}
      </Surface>
      <ChatDaySummaryDrawer
        chatId={chatId}
        chatTitle={title}
        date={summaryDate}
        onClose={closeDaySummary}
        open={Boolean(summaryDate)}
      />
    </DashboardPage>
  );
}

function ChatHistorySkeleton() {
  return (
    <div aria-label="正在加载聊天记录" className="chat-history-skeleton">
      {Array.from({ length: 6 }, (_, index) => (
        <div className="chat-history-skeleton-row" key={index}>
          <span />
          <div><i /><i /></div>
        </div>
      ))}
    </div>
  );
}

function prependUnique(earlier: ChatMessage[], current: ChatMessage[]) {
  const currentIDs = new Set(current.map((message) => message.id));
  return [...earlier.filter((message) => !currentIDs.has(message.id)), ...current];
}

function isAbortError(reason: unknown) {
  return reason instanceof DOMException && reason.name === "AbortError";
}

function asMessage(reason: unknown) {
  if (reason instanceof APIError || reason instanceof Error) return reason.message;
  return "服务器返回错误";
}

function BackIcon() {
  return (
    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24">
      <path d="m15 18-6-6 6-6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" />
    </svg>
  );
}

function SearchIcon() {
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24"><circle cx="10.8" cy="10.8" r="6.3" stroke="currentColor" strokeWidth="1.8" /><path d="m16 16 4 4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" /></svg>;
}

function SummaryIcon() {
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24"><path d="M6.5 3.8h8l3 3v13.4H6.5z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.7" /><path d="M14.5 3.8v3h3M9.3 11h5.4M9.3 14.2h5.4M9.3 17.4h3.2" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /></svg>;
}

function currentSearchParams() {
  return new URLSearchParams(window.location.search);
}

function pushSearchParams(pathname: string, params: URLSearchParams) {
  window.history.pushState(null, "", `${pathname}?${params.toString()}`);
}

function positiveSearchParam(value: string | null, fallback: number) {
  if (!value || !/^\d+$/.test(value)) return fallback;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}
