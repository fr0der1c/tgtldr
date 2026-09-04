"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Drawer } from "@/components/drawer";
import { EmptyState } from "@/components/dashboard-page";
import { SummaryMarkdown, type MarkdownSourceReference } from "@/components/summary-markdown";
import { StatusPill } from "@/components/ui";
import { statusText, statusTone } from "@/components/summaries-panel-sections";
import { useToast } from "@/components/toast";
import { api } from "@/lib/api";
import { DailyDigest, DailyDigestSource, Language, Summary } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

/** DailyDigestDrawer 在侧栏中展示每日总览历史、正文和来源摘要。 */
export function DailyDigestDrawer({
  botReady,
  onClose,
  onOpenSummary,
  open,
}: {
  botReady: boolean;
  onClose: () => void;
  onOpenSummary: (summary: Summary) => void;
  open: boolean;
}) {
  const [items, setItems] = useState<DailyDigest[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<DailyDigest | null>(null);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const toast = useToast();
  const { language } = useI18n();

  useEffect(() => {
    if (open) void loadHistory();
  }, [open]);

  const processing = useMemo(
    () => items.some((item) => item.status === "pending" || item.status === "running")
      || detail?.status === "pending"
      || detail?.status === "running",
    [detail?.status, items],
  );

  useEffect(() => {
    if (!open || !processing) return;
    const timer = window.setInterval(() => void refreshProcessing(), 3000);
    return () => window.clearInterval(timer);
  }, [open, processing, selectedId]);

  /** 刷新第一页，并优先保留用户正在查看的记录。 */
  async function loadHistory() {
    setLoading(true);
    try {
      const response = await api.listDailyDigests();
      setItems(response.items);
      setPage(1);
      setTotal(response.total);
      const nextId = selectedId ?? response.items[0]?.id ?? null;
      setSelectedId(nextId);
      setDetail(nextId ? await api.getDailyDigest(nextId) : null);
    } catch (error) {
      toast.showError(asMessage(error));
    } finally {
      setLoading(false);
    }
  }

  /** 轮询生成中的任务，并同步当前详情。 */
  async function refreshProcessing() {
    try {
      const response = await api.listDailyDigests();
      setItems((current) => {
        const refreshedIDs = new Set(response.items.map((item) => item.id));
        return [...response.items, ...current.filter((item) => !refreshedIDs.has(item.id))];
      });
      setTotal(response.total);
      if (selectedId) setDetail(await api.getDailyDigest(selectedId));
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 加载指定总览详情。 */
  async function selectDigest(id: number) {
    setSelectedId(id);
    setLoading(true);
    try {
      setDetail(await api.getDailyDigest(id));
    } catch (error) {
      toast.showError(asMessage(error));
    } finally {
      setLoading(false);
    }
  }

  /** 追加历史分页并按 ID 去重。 */
  async function loadMore() {
    try {
      const nextPage = page + 1;
      const response = await api.listDailyDigests(nextPage);
      setItems((current) => {
        const existingIDs = new Set(current.map((item) => item.id));
        return [...current, ...response.items.filter((item) => !existingIDs.has(item.id))];
      });
      setPage(nextPage);
      setTotal(response.total);
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 打开来源摘要前关闭当前侧栏，避免两个默认层级侧栏叠加。 */
  async function openSource(source: DailyDigestSource) {
    try {
      const summary = await api.getSummary(source.summaryId);
      onClose();
      onOpenSummary(summary);
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 使用原参与群组的最新摘要重新生成正文。 */
  async function rerun() {
    if (!detail) return;
    try {
      await api.rerunDailyDigest(detail.id);
      toast.showSuccess("已提交每日总览重新生成。完成后不会自动重复发送。");
      await refreshProcessing();
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 重新发送已经成功生成的每日总览。 */
  async function retryDelivery() {
    if (!detail) return;
    try {
      await api.retryDailyDigestDelivery(detail.id);
      toast.showSuccess("已重新提交 Telegram 发送。");
      setDetail(await api.getDailyDigest(detail.id));
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  return (
    <Drawer onClose={onClose} open={open} panelClassName="catch-up-drawer-panel" title="每日总览">
      {items.length === 0 && !loading ? (
        <EmptyState description="启用后，每天生成的总览会保存在这里。" title="没有可查看的每日总览" />
      ) : (
        <div className="catch-up-history-layout">
          <aside className="catch-up-history-list" aria-label="每日总览历史记录">
            {items.map((item) => (
              <button
                aria-pressed={selectedId === item.id}
                className={`catch-up-history-item${selectedId === item.id ? " selected" : ""}`}
                key={item.id}
                onClick={() => void selectDigest(item.id)}
                type="button"
              >
                <span className="catch-up-history-item-head">
                  <strong>{item.summaryDate}</strong>
                  <StatusPill tone={statusTone(item.status)}>{statusText(item.status)}</StatusPill>
                </span>
                <span>{formatSourceCount(item, language)}</span>
                <small>{formatDateTime(item.createdAt, language)}</small>
              </button>
            ))}
            {items.length < total ? (
              <button className="text-link-button catch-up-load-more" onClick={() => void loadMore()} type="button">
                加载更多
              </button>
            ) : null}
          </aside>

          <main className="catch-up-detail">
            {loading && !detail ? (
              <div className="catch-up-inline-loading"><span className="catch-up-spinner" />正在读取…</div>
            ) : detail ? (
              <DailyDigestDetail
                item={detail}
                botReady={botReady}
                language={language}
                onOpenSource={openSource}
                onRerun={rerun}
                onRetryDelivery={retryDelivery}
              />
            ) : (
              <EmptyState description="启用后，每天生成的总览会保存在这里。" title="没有可查看的每日总览" />
            )}
          </main>
        </div>
      )}
    </Drawer>
  );
}

/** DailyDigestDetail 展示总览状态、正文、来源和人工操作。 */
function DailyDigestDetail({
  botReady,
  item,
  language,
  onOpenSource,
  onRerun,
  onRetryDelivery,
}: {
  botReady: boolean;
  item: DailyDigest;
  language: Language;
  onOpenSource: (source: DailyDigestSource) => void;
  onRerun: () => void;
  onRetryDelivery: () => void;
}) {
  const includedSources = useMemo(
    () => addSourceReferences((item.sources ?? []).filter((source) => source.included)),
    [item.sources],
  );
  const omittedSources = useMemo(
    () => (item.sources ?? []).filter((source) => source.omissionReason && source.omissionReason !== "no_messages"),
    [item.sources],
  );
  const sourceByReference = useMemo(
    () => new Map(includedSources.map((source) => [source.reference, source])),
    [includedSources],
  );
  const markdownSources = useMemo<ReadonlyMap<string, MarkdownSourceReference>>(
    () => new Map(includedSources.map((source) => [source.reference, {
      label: String(source.number),
      title: `${source.chatTitle} · ${item.summaryDate}`,
      ariaLabel: language === "en"
        ? `Open source ${source.number}: ${source.chatTitle}`
        : `打开来源 ${source.number}：${source.chatTitle}`,
    }])),
    [includedSources, item.summaryDate, language],
  );
  const openSourceReference = useCallback((reference: string) => {
    const source = sourceByReference.get(reference);
    if (source) onOpenSource(source);
  }, [onOpenSource, sourceByReference]);
  const delivery = dailyDigestDeliveryState(item, botReady);
  const processing = item.status === "pending" || item.status === "running";

  return (
    <div className="catch-up-detail-stack">
      <header className="catch-up-detail-head">
        <div>
          <p className="dashboard-page-kicker">每日总览</p>
          <h2>{item.summaryDate}</h2>
          <p>{formatSourceCount(item, language)}</p>
        </div>
        <div className="summary-status-actions">
          <StatusPill tone={statusTone(item.status)}>{statusText(item.status)}</StatusPill>
          <StatusPill tone={delivery.tone} title={delivery.detail}>{delivery.label}</StatusPill>
          {!processing ? <button className="text-link-button" onClick={onRerun} type="button">重新生成</button> : null}
          {botReady && item.status === "succeeded" && !item.deliveredAt && !item.deliverySkippedReason ? (
            <button className="text-link-button" onClick={onRetryDelivery} type="button">通过 Bot 发送</button>
          ) : null}
        </div>
      </header>

      {processing ? (
        <div aria-live="polite" className="catch-up-generating compact" role="status">
          <span className="catch-up-spinner" />
          <h3>正在生成每日总览…</h3>
          <p>任务会在后台继续，完成后这里会自动刷新。</p>
        </div>
      ) : item.status === "failed" ? (
        <div className="catch-up-error">
          <strong>生成失败</strong>
          <pre>{item.errorMessage}</pre>
          {item.nextRetryAt ? <span>下次自动重试：{formatDateTime(item.nextRetryAt, language)}</span> : null}
        </div>
      ) : (
        <div className="summary-detail-content">
          <SummaryMarkdown
            content={item.content}
            onSourceReferenceClick={openSourceReference}
            sourceReferences={markdownSources}
          />
        </div>
      )}

      {!processing && (includedSources.length > 0 || omittedSources.length > 0 || item.emptyChatCount > 0) ? (
        <section className="catch-up-sources">
          <div>
            <h3>来源摘要</h3>
            <p>点击群组可打开对应的单群每日摘要。</p>
          </div>
          {includedSources.length > 0 ? (
            <div className="catch-up-source-links daily-digest-source-links">
              {includedSources.map((source) => (
                <button key={source.summaryId} onClick={() => onOpenSource(source)} type="button">
                  {source.number} · {source.chatTitle}
                </button>
              ))}
            </div>
          ) : null}
          {item.emptyChatCount > 0 ? <p>{item.emptyChatCount} 个群组当天没有新消息。</p> : null}
          {omittedSources.length > 0 ? (
            <div className="daily-digest-omissions">
              <strong>未纳入</strong>
              <span>{omittedSources.map((source) => source.chatTitle).join(language === "en" ? ", " : "、")}</span>
            </div>
          ) : null}
        </section>
      ) : null}
    </div>
  );
}

type ReferencedSource = DailyDigestSource & { reference: string; number: number };

/** 按模型输入顺序补充来源编号。 */
function addSourceReferences(sources: DailyDigestSource[]): ReferencedSource[] {
  return sources.map((source, index) => ({
    ...source,
    number: index + 1,
    reference: `S${String(index + 1).padStart(3, "0")}`,
  }));
}

/** 将持久化投递字段转换成界面状态。 */
function dailyDigestDeliveryState(item: DailyDigest, botReady: boolean): { label: string; tone: "neutral" | "good" | "warn" | "bad"; detail?: string } {
  if (item.deliverySkippedReason) return { label: "无需发送", tone: "neutral", detail: "当天没有可推送的新内容。" };
  if (item.deliveredAt) return { label: "已发送", tone: "good" };
  if (item.deliveryError) return { label: "发送失败", tone: "bad", detail: item.deliveryError };
  if (item.status !== "succeeded") return { label: "未发送", tone: "neutral" };
  if (item.deliverySuppressed) return { label: "等待手动发送", tone: "warn", detail: "重新生成完成后不会自动重复发送。" };
  if (!botReady) return { label: "等待 Bot 配置", tone: "warn", detail: "Bot 配置尚未完成，当前无法发送。" };
  return { label: "待发送", tone: "warn" };
}

function formatSourceCount(item: DailyDigest, language: Language) {
  return language === "en"
    ? `${item.participantCount} chats · ${item.sourceSummaryCount} summaries`
    : `${item.participantCount} 个群组 · ${item.sourceSummaryCount} 份摘要`;
}

function formatDateTime(value: string, language: Language) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(language, { hour12: false });
}

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
