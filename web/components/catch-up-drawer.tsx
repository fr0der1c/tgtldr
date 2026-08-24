"use client";

import { useEffect, useMemo, useState } from "react";
import { Drawer } from "@/components/drawer";
import { EmptyState } from "@/components/dashboard-page";
import { SummaryMarkdown } from "@/components/summary-markdown";
import { StatusPill } from "@/components/ui";
import { statusText, statusTone } from "@/components/summaries-panel-sections";
import { useToast } from "@/components/toast";
import { api } from "@/lib/api";
import { CatchUp, CatchUpSource, Summary } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

/** CatchUpDrawer 在同一侧栏中组合历史列表、任务状态、正文和来源入口。 */
export function CatchUpDrawer({
  initialCatchUpId,
  onClose,
  onOpenSummary,
  open,
}: {
  initialCatchUpId: number | null;
  onClose: () => void;
  onOpenSummary: (summary: Summary) => void;
  open: boolean;
}) {
  const [items, setItems] = useState<CatchUp[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<CatchUp | null>(null);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const toast = useToast();
  const { language } = useI18n();

  useEffect(() => {
    if (!open) return;
    void loadHistory(initialCatchUpId);
  }, [initialCatchUpId, open]);

  const processing = useMemo(
    () => items.some((item) => item.status === "pending" || item.status === "running"),
    [items],
  );

  useEffect(() => {
    if (!open || !processing) return;
    const timer = window.setInterval(() => void refreshProcessing(), 3000);
    return () => window.clearInterval(timer);
  }, [open, processing, selectedId]);

  /** 刷新历史列表，并优先打开刚完成或用户指定的记录。 */
  async function loadHistory(preferredId: number | null) {
    setLoading(true);
    try {
      const response = await api.listCatchUps();
      setItems(response.items);
      setPage(1);
      setTotal(response.total);
      const nextId = preferredId ?? selectedId ?? response.items[0]?.id ?? null;
      setSelectedId(nextId);
      setDetail(nextId ? await api.getCatchUp(nextId) : null);
    } catch (error) {
      toast.showError(asMessage(error));
    } finally {
      setLoading(false);
    }
  }

  /** 轮询只更新进行中的记录，完成后同时刷新详情。 */
  async function refreshProcessing() {
    try {
      const response = await api.listCatchUps();
      setItems((current) => {
        const refreshedIds = new Set(response.items.map((item) => item.id));
        return [...response.items, ...current.filter((item) => !refreshedIds.has(item.id))];
      });
      setTotal(response.total);
      if (selectedId) setDetail(await api.getCatchUp(selectedId));
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 切换历史记录时只加载所选详情，列表保持当前分页。 */
  async function selectCatchUp(id: number) {
    setSelectedId(id);
    setLoading(true);
    try {
      setDetail(await api.getCatchUp(id));
    } catch (error) {
      toast.showError(asMessage(error));
    } finally {
      setLoading(false);
    }
  }

  /** 追加下一页并按 ID 去重，避免分页期间新增任务造成重复项。 */
  async function loadMore() {
    try {
      const nextPage = page + 1;
      const response = await api.listCatchUps(nextPage);
      setItems((current) => {
        const existingIds = new Set(current.map((item) => item.id));
        return [...current, ...response.items.filter((item) => !existingIds.has(item.id))];
      });
      setPage(nextPage);
      setTotal(response.total);
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 打开来源摘要前关闭 Catch Up，避免同时叠加两个默认层级 Drawer。 */
  async function openSource(source: CatchUpSource) {
    try {
      const summary = await api.getSummary(source.summaryId);
      onClose();
      onOpenSummary(summary);
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  /** 重新投递只更新 Telegram 状态，不重新生成正文。 */
  async function retryDelivery() {
    if (!detail) return;
    try {
      await api.retryCatchUpDelivery(detail.id);
      toast.showSuccess("已重新提交 Telegram 发送。");
      setDetail(await api.getCatchUp(detail.id));
    } catch (error) {
      toast.showError(asMessage(error));
    }
  }

  return (
    <Drawer
      onClose={onClose}
      open={open}
      panelClassName="catch-up-drawer-panel"
      title="快速回顾"
    >
      <div className="catch-up-history-layout">
        <aside className="catch-up-history-list" aria-label="快速回顾历史记录">
          {items.length === 0 && !loading ? (
            <p className="muted">还没有生成过快速回顾。</p>
          ) : (
            <>
              {items.map((item) => (
                <button
                  aria-pressed={selectedId === item.id}
                  className={`catch-up-history-item${selectedId === item.id ? " selected" : ""}`}
                  key={item.id}
                  onClick={() => void selectCatchUp(item.id)}
                  type="button"
                >
                  <span className="catch-up-history-item-head">
                    <strong>{item.fromDate} – {item.toDate}</strong>
                    <StatusPill tone={statusTone(item.status)}>{statusText(item.status)}</StatusPill>
                  </span>
                  <span>{language === "en"
                    ? `${item.chatCount} chats · ${item.sourceSummaryCount} summaries`
                    : `${item.chatCount} 个群组 · ${item.sourceSummaryCount} 份摘要`}</span>
                  <small>{formatDateTime(item.createdAt, language)}</small>
                </button>
              ))}
              {items.length < total ? (
                <button className="text-link-button catch-up-load-more" onClick={() => void loadMore()} type="button">
                  加载更多
                </button>
              ) : null}
            </>
          )}
        </aside>

        <main className="catch-up-detail">
          {loading && !detail ? (
            <div className="catch-up-inline-loading"><span className="catch-up-spinner" />正在读取…</div>
          ) : detail ? (
            <CatchUpDetail
              item={detail}
              language={language}
              onOpenSource={openSource}
              onRetryDelivery={retryDelivery}
            />
          ) : (
            <EmptyState description="生成完成后，可以在这里查看完整阶段回顾。" title="没有可查看的快速回顾" />
          )}
        </main>
      </div>
    </Drawer>
  );
}

/** CatchUpDetail 按任务状态展示等待、错误或成功正文，并提供来源索引。 */
function CatchUpDetail({
  item,
  language,
  onOpenSource,
  onRetryDelivery,
}: {
  item: CatchUp;
  language: "zh-CN" | "en";
  onOpenSource: (source: CatchUpSource) => void;
  onRetryDelivery: () => void;
}) {
  const sourcesByChat = useMemo(() => groupSources(item.sources ?? []), [item.sources]);
  const delivery = catchUpDeliveryState(item);

  return (
    <div className="catch-up-detail-stack">
      <header className="catch-up-detail-head">
        <div>
          <p className="dashboard-page-kicker">快速回顾</p>
          <h2>{item.fromDate} – {item.toDate}</h2>
          <p>{language === "en"
            ? `${item.chatCount} chats · ${item.sourceSummaryCount} daily summaries`
            : `${item.chatCount} 个群组 · ${item.sourceSummaryCount} 份每日摘要`}</p>
        </div>
        <div className="summary-status-actions">
          <StatusPill tone={statusTone(item.status)}>{statusText(item.status)}</StatusPill>
          {item.deliveryRequested ? <StatusPill tone={delivery.tone} title={delivery.detail}>{delivery.label}</StatusPill> : null}
          {item.status === "succeeded" && item.deliveryRequested && !item.deliveredAt ? (
            <button className="text-link-button" onClick={onRetryDelivery} type="button">重新发送</button>
          ) : null}
        </div>
      </header>

      {item.status === "pending" || item.status === "running" ? (
        <div aria-live="polite" className="catch-up-generating compact" role="status">
          <span className="catch-up-spinner" />
          <h3>正在生成快速回顾…</h3>
          <p>任务会在后台继续，完成后这里会自动刷新。</p>
        </div>
      ) : item.status === "failed" ? (
        <div className="catch-up-error">
          <strong>生成失败</strong>
          <pre>{item.errorMessage}</pre>
        </div>
      ) : (
        <div className="summary-detail-content">
          <SummaryMarkdown content={item.content} />
        </div>
      )}

      {item.status === "succeeded" && sourcesByChat.length > 0 ? (
        <section className="catch-up-sources">
          <div>
            <h3>来源摘要</h3>
            <p>点击日期可打开对应的每日摘要。</p>
          </div>
          {sourcesByChat.map(([chatTitle, sources]) => (
            <details key={chatTitle}>
              <summary>{chatTitle}<span>{language === "en" ? `${sources.length} summaries` : `${sources.length} 份`}</span></summary>
              <div className="catch-up-source-links">
                {sources.map((source) => (
                  <button key={source.summaryId} onClick={() => onOpenSource(source)} type="button">
                    {source.reference} · {source.summaryDate}
                  </button>
                ))}
              </div>
            </details>
          ))}
        </section>
      ) : null}
    </div>
  );
}

type ReferencedSource = CatchUpSource & { reference: string };

/** groupSources 保留全局来源编号，同时按群组组织可展开列表。 */
function groupSources(sources: CatchUpSource[]) {
  const grouped = new Map<string, ReferencedSource[]>();
  for (const [index, source] of sources.entries()) {
    const current = grouped.get(source.chatTitle) ?? [];
    current.push({ ...source, reference: `S${String(index + 1).padStart(3, "0")}` });
    grouped.set(source.chatTitle, current);
  }
  return Array.from(grouped.entries());
}

function catchUpDeliveryState(item: CatchUp): { label: string; tone: "neutral" | "good" | "warn" | "bad"; detail?: string } {
  if (item.deliveredAt) return { label: "已发送", tone: "good" };
  if (item.deliveryError) return { label: "发送失败", tone: "bad", detail: item.deliveryError };
  return { label: "待发送", tone: "warn" };
}

function formatDateTime(value: string, language: "zh-CN" | "en") {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(language, { hour12: false });
}

function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
