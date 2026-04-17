"use client";

import { startTransition, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import { AppSelect } from "@/components/app-select";
import { SearchSelect } from "@/components/search-select";
import { Chat, Summary, SummaryContextPreview } from "@/lib/types";
import { DashboardPage, EmptyState, MetricCard, MetricRail, Surface } from "@/components/dashboard-page";
import { SummaryContextModal } from "@/components/summary-context-modal";
import { SummaryMarkdown } from "@/components/summary-markdown";
import { useToast } from "@/components/toast";
import { Button, Field, Input, StatusPill } from "@/components/ui";

type SummaryFilter = "all" | Summary["status"];
type DeliveryTone = "neutral" | "good" | "warn" | "bad";
type DeliveryState = {
  label: string;
  tone: DeliveryTone;
  detail?: string;
  retryable: boolean;
};

export function SummariesPanel() {
	const [summaries, setSummaries] = useState<Summary[]>([]);
  const [allChats, setAllChats] = useState<Chat[]>([]);
  const [chats, setChats] = useState<Chat[]>([]);
  const [botReady, setBotReady] = useState(false);
  const [selectedSummaryId, setSelectedSummaryId] = useState<number | null>(null);
  const [selectedChatId, setSelectedChatId] = useState("");
  const [manualDate, setManualDate] = useState(localDateInputValue());
  const [filter, setFilter] = useState<SummaryFilter>("all");
  const [manualEditorOpen, setManualEditorOpen] = useState(false);
  const [contextOpen, setContextOpen] = useState(false);
	const [contextPreview, setContextPreview] = useState<SummaryContextPreview | null>(null);
	const [contextPreviewSummaryID, setContextPreviewSummaryID] = useState<number | null>(null);
	const [contextLoading, setContextLoading] = useState(false);
	const loadRef = useRef<() => Promise<void>>(async () => {});
	const toast = useToast();

	useEffect(() => {
		void loadRef.current();
	}, []);

	useEffect(() => {
    if (!selectedSummaryId && summaries.length > 0) {
      setSelectedSummaryId(summaries[0].id);
    }
  }, [selectedSummaryId, summaries]);

  useEffect(() => {
    if (!contextOpen || !selectedSummaryId) {
      setContextPreview(null);
      return;
    }
    if (contextPreviewSummaryID === selectedSummaryId && contextPreview) {
      return;
    }
    let cancelled = false;
    setContextLoading(true);
    void api
      .summaryContextPreview(selectedSummaryId)
      .then((preview) => {
        if (cancelled) {
          return;
        }
        setContextPreview(preview);
        setContextPreviewSummaryID(selectedSummaryId);
      })
      .catch((err) => {
        if (cancelled) {
          return;
        }
        setContextPreview(null);
        toast.showError(asMessage(err));
      })
      .finally(() => {
        if (cancelled) {
          return;
        }
        setContextLoading(false);
      });

    return () => {
      cancelled = true;
    };
	}, [contextOpen, contextPreview, contextPreviewSummaryID, selectedSummaryId, toast]);

	const hasActiveSummaries = useMemo(() => {
		return summaries.some((item) => item.status === "pending" || item.status === "running");
	}, [summaries]);

	async function load() {
    try {
      const [summaryData, chatData, settingsData] = await Promise.all([
        api.listSummaries(),
        api.listChats(),
        api.settings()
      ]);
      const manualChats = chatData.filter((chat) => chat.summaryEnabled);
      setSummaries(summaryData);
      setAllChats(chatData);
      setChats(manualChats);
      setBotReady(
        settingsData.botEnabled &&
          Boolean(settingsData.botToken?.trim()) &&
          Boolean(settingsData.botTargetChatId?.trim())
      );
      setSelectedSummaryId((current) =>
        current && summaryData.some((item) => item.id === current)
          ? current
          : summaryData[0]?.id ?? null
      );
      setSelectedChatId((current) => {
        if (current && manualChats.some((chat) => String(chat.id) === current)) {
          return current;
        }
        return manualChats[0] ? String(manualChats[0].id) : "";
      });
    } catch (err) {
      toast.showError(asMessage(err));
		}
	}
	loadRef.current = load;

	useEffect(() => {
		if (!hasActiveSummaries) {
			return;
		}
		const timer = window.setInterval(() => {
			void loadRef.current();
		}, 3000);
		return () => window.clearInterval(timer);
	}, [hasActiveSummaries]);

  async function runManual() {
    if (!selectedChatId || !manualDate) {
      return;
    }

    try {
		await api.runSummary(Number(selectedChatId), manualDate);
		toast.showSuccess("已提交摘要生成任务。");
		setManualEditorOpen(false);
		await loadRef.current();
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  async function retryDelivery(summary: Summary) {
    try {
      await api.retrySummaryDelivery(summary.id);
      toast.showSuccess("已提交通过 Bot 发送。");
      await loadRef.current();
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  const chatTitles = useMemo(() => {
    return new Map(allChats.map((chat) => [chat.id, chat.title]));
  }, [allChats]);

  const filtered = useMemo(() => {
    if (filter === "all") {
      return summaries;
    }
    return summaries.filter((item) => item.status === filter);
  }, [filter, summaries]);

  const selectedSummary = useMemo(
    () => summaries.find((item) => item.id === selectedSummaryId) ?? null,
    [selectedSummaryId, summaries]
  );
  const selectedChat = useMemo(
    () => (selectedSummary ? allChats.find((item) => item.id === selectedSummary.chatId) ?? null : null),
    [allChats, selectedSummary]
  );
  const selectedDelivery = useMemo(
    () => (selectedSummary ? deliveryState(selectedSummary, selectedChat, botReady) : null),
    [botReady, selectedChat, selectedSummary]
  );

  const successCount = summaries.filter((item) => item.status === "succeeded").length;
  const failedCount = summaries.filter((item) => item.status === "failed").length;
  const runningCount = summaries.filter((item) => item.status === "running").length;

  return (
    <DashboardPage
      title="摘要"
      description="在这里查看每天生成的摘要结果，筛选状态，并在需要时手动补跑。"
    >
      <MetricRail>
        <MetricCard
          label="摘要记录"
          value={summaries.length}
          badge="累计"
          detail="已经写入数据库的摘要任务与结果。"
        />
        <MetricCard
          label="生成成功"
          value={successCount}
          tone={successCount > 0 ? "good" : "neutral"}
          badge={successCount > 0 ? "正常" : "暂无"}
          detail="状态为 succeeded 的摘要数量。"
        />
        <MetricCard
          label="处理中"
          value={runningCount}
          tone={runningCount > 0 ? "warn" : "neutral"}
          badge={runningCount > 0 ? "进行中" : "空闲"}
          detail="当前正在运行或等待完成的摘要。"
        />
        <MetricCard
          label="生成失败"
          value={failedCount}
          tone={failedCount > 0 ? "bad" : "good"}
          badge={failedCount > 0 ? "需排查" : "稳定"}
          detail="失败任务建议重新执行，并检查模型配置或上下文限制。"
        />
      </MetricRail>

      <div className="dashboard-workspace summary-workspace">
        <Surface
          title="摘要记录"
          description="左侧查看历史摘要，按需展开手动补跑；右侧查看正文和原始 prompt。"
        >
			<div className="summary-toolbar">
				<div className="summary-filter">
					<Field label="状态筛选">
						<AppSelect
							onChange={(value) => setFilter(value as SummaryFilter)}
							options={[
								{ value: "all", label: "全部状态" },
								{ value: "succeeded", label: "成功" },
								{ value: "running", label: "运行中" },
								{ value: "pending", label: "等待中" },
								{ value: "failed", label: "失败" }
							]}
							value={filter}
						/>
					</Field>
				</div>
				<Button
					className="summary-toolbar-button"
					onClick={() => setManualEditorOpen((current) => !current)}
					type="button"
				>
					{manualEditorOpen ? "收起补跑" : "手动补跑"}
				</Button>
			</div>

			{manualEditorOpen ? (
				<div className="summary-manual-panel">
              <div className="summary-manual-head">
                <strong>手动补跑</strong>
                <p>只会显示已启用 AI 总结的群组。</p>
              </div>
              {chats.length === 0 ? (
                <EmptyState
                  title="还没有可补跑的群组"
                  description="只有已启用 AI 总结的群组才会出现在这里。"
                />
              ) : (
                <>
                  <div className="form-grid">
                    <Field label="群组">
                      <SearchSelect
                        emptyText="没有匹配的群组"
                        onChange={setSelectedChatId}
                        options={chats.map((chat) => ({
                          value: String(chat.id),
                          label: chat.title
                        }))}
                        placeholder="选择群组"
                        searchPlaceholder="搜索群组"
                        value={selectedChatId}
                      />
                    </Field>
                    <Field label="日期">
                      <Input
                        type="date"
                        value={manualDate}
                        onChange={(event) => setManualDate(event.target.value)}
                      />
                    </Field>
                  </div>
                  <div className="summary-manual-actions">
                    <Button
                      onClick={() => startTransition(() => void runManual())}
                      type="button"
                    >
                      立即生成
                    </Button>
                  </div>
                </>
              )}
            </div>
          ) : null}

          {filtered.length === 0 ? (
            <EmptyState
              title="还没有摘要记录"
              description="先展开手动补跑触发一次摘要，或者等待定时任务执行。"
            />
          ) : (
            <div className="entity-list">
              {filtered.map((item) => {
                const delivery = deliveryState(
                  item,
                  allChats.find((chat) => chat.id === item.chatId) ?? null,
                  botReady
                );
                return (
                  <button
                    key={item.id}
                    className={`entity-row ${item.id === selectedSummaryId ? "active" : ""}`}
                    onClick={() => setSelectedSummaryId(item.id)}
                    type="button"
                  >
                    <div className="entity-row-main">
                      <strong>{chatTitles.get(item.chatId) ?? "未知群组"}</strong>
                      <p>
                        {item.summaryDate} · {item.model || "未记录模型"}
                      </p>
                    </div>
                    <div className="entity-row-meta">
                      <StatusPill tone={statusTone(item.status)}>{statusText(item.status)}</StatusPill>
                      <StatusPill
                        className={delivery.detail ? "status-pill-hoverable" : undefined}
                        title={delivery.detail}
                        tone={delivery.tone}
                      >
                        {delivery.label}
                      </StatusPill>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </Surface>

        <div className="summary-detail-column">
          <Surface
            title={selectedSummary ? undefined : "摘要内容"}
          >
            {!selectedSummary ? (
              <EmptyState
                title="没有可查看的摘要"
                description="先生成摘要，或从左侧列表中选择一条已有记录。"
              />
            ) : (
              <div className="summary-detail-stack">
                <div className="summary-detail-header">
                  <h2>
                    {chatTitles.get(selectedSummary.chatId) ?? "未知群组"} · {selectedSummary.summaryDate}
                  </h2>
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
                        onClick={() => startTransition(() => void retryDelivery(selectedSummary))}
                        type="button"
                      >
                        通过 Bot 发送
                      </button>
                    ) : null}
                  </div>
                </div>
                <div className="summary-detail-meta">
                  <p>
                    {selectedSummary.model || "未记录模型"} · 消息{" "}
                    {selectedSummary.sourceMessageCount} 条 · 分块{" "}
                    {selectedSummary.chunkCount}
                  </p>
                  <button
                    className="text-link-button"
                    onClick={() => setContextOpen(true)}
                    type="button"
                  >
                    查看原始 prompt
                  </button>
                </div>
                <SummaryContent summary={selectedSummary} />
              </div>
            )}
          </Surface>
        </div>
      </div>

      <SummaryContextModal
        loading={contextLoading}
        onClose={() => setContextOpen(false)}
        open={contextOpen}
        preview={contextPreview}
      />
    </DashboardPage>
  );
}

function SummaryContent({ summary }: { summary: Summary }) {
  if (summary.status === "failed") {
    return <pre className="summary-context-block">{summary.errorMessage || ""}</pre>;
  }

  return <SummaryMarkdown content={summary.content || ""} />;
}

function asMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function statusTone(status: Summary["status"]) {
  if (status === "succeeded") return "good";
  if (status === "failed") return "bad";
  if (status === "running") return "warn";
  return "neutral";
}

function statusText(status: Summary["status"]) {
  if (status === "succeeded") return "成功";
  if (status === "failed") return "失败";
  if (status === "running") return "运行中";
  return "等待中";
}

function deliveryState(summary: Summary, chat: Chat | null, botReady: boolean): DeliveryState {
  if (!chat || chat.deliveryMode !== "bot") {
    return {
      label: "不发送",
      tone: "neutral",
      detail: "当前群组设置为不通过 Bot 推送。",
      retryable: false
    };
  }
  if (!botReady) {
    return {
      label: "待发送",
      tone: "warn",
      detail: "Bot 配置尚未完成，当前无法发送。",
      retryable: false
    };
  }
  if (summary.status !== "succeeded") {
    return {
      label: "未发送",
      tone: "neutral",
      detail: "摘要尚未生成成功，当前不会执行发送。",
      retryable: false
    };
  }
  if (summary.deliveredAt) {
    return {
      label: "已发送",
      tone: "good",
      detail: `已发送于 ${summary.deliveredAt}`,
      retryable: false
    };
  }
  if (summary.deliveryError) {
    return {
      label: "发送失败",
      tone: "bad",
      detail: summary.deliveryError,
      retryable: true
    };
  }
  return {
    label: "待发送",
    tone: "warn",
    detail: "摘要已生成，等待自动发送或手动重试。",
    retryable: true
  };
}

function localDateInputValue() {
  const now = new Date();
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}
