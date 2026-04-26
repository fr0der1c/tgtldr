"use client";

import { startTransition, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import {
	Chat,
	Summary,
	SummaryContextPreview,
	SummarySearchFilters,
	SummaryStats,
} from "@/lib/types";
import {
	DashboardPage,
	MetricCard,
	MetricRail,
} from "@/components/dashboard-page";
import { SummaryContextModal } from "@/components/summary-context-modal";
import {
	DeliveryFilter,
	SummaryFilter,
	SummaryListSection,
	localDateInputValue,
} from "@/components/summaries-panel-sections";
import { SummaryDetailDrawer } from "@/components/summary-detail-drawer";
import { useToast } from "@/components/toast";

const summaryPageSize = 20;

export function SummariesPanel() {
	const [summaries, setSummaries] = useState<Summary[]>([]);
	const [summaryStats, setSummaryStats] = useState<SummaryStats>({
		total: 0,
		successCount: 0,
		processingCount: 0,
		failedCount: 0,
	});
	const [allChats, setAllChats] = useState<Chat[]>([]);
	const [chats, setChats] = useState<Chat[]>([]);
	const [botReady, setBotReady] = useState(false);
	const [selectedSummaryId, setSelectedSummaryId] = useState<number | null>(null);
	const [detailOpen, setDetailOpen] = useState(false);
	const [selectedChatId, setSelectedChatId] = useState("");
	const [manualDate, setManualDate] = useState(localDateInputValue());
	const [filter, setFilter] = useState<SummaryFilter>("all");
	const [deliveryFilter, setDeliveryFilter] = useState<DeliveryFilter>("all");
	const [query, setQuery] = useState("");
	const [chatFilter, setChatFilter] = useState("all");
	const [dateFrom, setDateFrom] = useState("");
	const [dateTo, setDateTo] = useState("");
	const [page, setPage] = useState(1);
	const [pageSize] = useState(summaryPageSize);
	const [total, setTotal] = useState(0);
	const [manualEditorOpen, setManualEditorOpen] = useState(false);
	const [contextOpen, setContextOpen] = useState(false);
	const [contextPreview, setContextPreview] = useState<SummaryContextPreview | null>(null);
	const [contextPreviewSummaryID, setContextPreviewSummaryID] = useState<number | null>(null);
	const [contextLoading, setContextLoading] = useState(false);
	const deferredQuery = useDeferredValue(query);
	const loadRef = useRef<() => Promise<void>>(async () => {});
	const loadStatsRef = useRef<() => Promise<void>>(async () => {});
	const toast = useToast();

	const searchFilters = useMemo<SummarySearchFilters>(
		() => ({
			q: deferredQuery.trim(),
			chatId: chatFilter,
			status: filter,
			delivery: deliveryFilter,
			dateFrom,
			dateTo,
			page,
			pageSize,
		}),
		[chatFilter, dateFrom, dateTo, deferredQuery, deliveryFilter, filter, page, pageSize],
	);

	useEffect(() => {
		void loadMeta();
		void loadStats();
	}, []);

	useEffect(() => {
		void loadRef.current();
	}, [searchFilters]);

	useEffect(() => {
		setPage(1);
	}, [deferredQuery, chatFilter, filter, deliveryFilter, dateFrom, dateTo]);

	useEffect(() => {
		if (summaries.length === 0) {
			setSelectedSummaryId(null);
			setDetailOpen(false);
			return;
		}
		if (!selectedSummaryId) {
			return;
		}
		if (!summaries.some((item) => item.id === selectedSummaryId)) {
			setSelectedSummaryId(null);
			setDetailOpen(false);
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

	const searchTerms = useMemo(() => {
		return deferredQuery
			.trim()
			.split(/\s+/)
			.map((item) => item.trim())
			.filter(Boolean);
	}, [deferredQuery]);

	const hasActiveSummaries = useMemo(() => {
		return (
			summaryStats.processingCount > 0 ||
			summaries.some((item) => item.status === "pending" || item.status === "running")
		);
	}, [summaries, summaryStats.processingCount]);

	async function loadMeta() {
		try {
			const [chatData, settingsData] = await Promise.all([api.listChats(), api.settings()]);
			const manualChats = chatData.filter((chat) => chat.summaryEnabled);
			setAllChats(chatData);
			setChats(manualChats);
			setBotReady(
				settingsData.botEnabled &&
					Boolean(settingsData.botToken?.trim()) &&
					Boolean(settingsData.botTargetChatId?.trim()),
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

	async function loadStats() {
		try {
			setSummaryStats(await api.summaryStats());
		} catch (err) {
			toast.showError(asMessage(err));
		}
	}

	async function loadSummaries(filters: SummarySearchFilters) {
		try {
			const response = await api.listSummaries(filters);
			setSummaries(response.items);
			setTotal(response.total);
			setSelectedSummaryId((current) =>
				current && response.items.some((item) => item.id === current)
					? current
					: response.items[0]?.id ?? null,
			);
		} catch (err) {
			toast.showError(asMessage(err));
		}
	}
	loadRef.current = async () => loadSummaries(searchFilters);
	loadStatsRef.current = loadStats;

	useEffect(() => {
		if (!hasActiveSummaries) {
			return;
		}
		const timer = window.setInterval(() => {
			void loadRef.current();
			void loadStatsRef.current();
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
			await Promise.all([loadRef.current(), loadStatsRef.current()]);
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

	async function rerunSummary(summary: Summary) {
		try {
			await api.runSummary(summary.chatId, summary.summaryDate);
			toast.showSuccess("已提交重新生成。");
			await Promise.all([loadRef.current(), loadStatsRef.current()]);
		} catch (err) {
			toast.showError(asMessage(err));
		}
	}

	const chatTitles = useMemo(() => {
		return new Map(allChats.map((chat) => [chat.id, chat.title]));
	}, [allChats]);

	const selectedSummary = useMemo(
		() => summaries.find((item) => item.id === selectedSummaryId) ?? null,
		[selectedSummaryId, summaries],
	);
	const selectedChat = useMemo(
		() => (selectedSummary ? allChats.find((item) => item.id === selectedSummary.chatId) ?? null : null),
		[allChats, selectedSummary],
	);

	const totalPages = total > 0 ? Math.ceil(total / pageSize) : 1;
	const searching = Boolean(
		deferredQuery.trim() || chatFilter !== "all" || filter !== "all" || deliveryFilter !== "all" || dateFrom || dateTo,
	);

	return (
		<DashboardPage
			description="在这里搜索历史摘要、筛选状态，并在需要时手动补跑。"
			title="摘要"
		>
			<MetricRail>
				<MetricCard badge="累计" detail="已经写入数据库的摘要任务与结果。" label="摘要记录" value={summaryStats.total} />
				<MetricCard
					badge={summaryStats.successCount > 0 ? "正常" : "暂无"}
					detail="状态为 succeeded 的摘要数量。"
					label="生成成功"
					tone={summaryStats.successCount > 0 ? "good" : "neutral"}
					value={summaryStats.successCount}
				/>
				<MetricCard
					badge={summaryStats.processingCount > 0 ? "进行中" : "空闲"}
					detail="当前正在运行或等待完成的摘要。"
					label="处理中"
					tone={summaryStats.processingCount > 0 ? "warn" : "neutral"}
					value={summaryStats.processingCount}
				/>
				<MetricCard
					badge={summaryStats.failedCount > 0 ? "需排查" : "稳定"}
					detail="失败任务建议重新执行，并检查模型配置或上下文限制。"
					label="生成失败"
					tone={summaryStats.failedCount > 0 ? "bad" : "good"}
					value={summaryStats.failedCount}
				/>
			</MetricRail>

			<SummaryListSection
				allChats={allChats}
				botReady={botReady}
				chatFilter={chatFilter}
				chatTitles={chatTitles}
				chats={chats}
				dateFrom={dateFrom}
				dateTo={dateTo}
				deliveryFilter={deliveryFilter}
				filter={filter}
				loadSummaryDate={manualDate}
				manualEditorOpen={manualEditorOpen}
				onChatFilterChange={setChatFilter}
				onDateFromChange={setDateFrom}
				onDateToChange={setDateTo}
				onDeliveryFilterChange={setDeliveryFilter}
				onFilterChange={setFilter}
				onLoadSummaryDateChange={setManualDate}
				onManualEditorToggle={() => setManualEditorOpen((current) => !current)}
				onManualRun={runManual}
				onPageChange={setPage}
				onQueryChange={setQuery}
				onSelectedChatChange={setSelectedChatId}
				onSelectSummary={(summaryId) => {
					setSelectedSummaryId(summaryId);
					setDetailOpen(true);
				}}
				page={page}
				query={query}
				searchTerms={searchTerms}
				searching={searching}
				selectedChatId={selectedChatId}
				selectedSummaryId={selectedSummaryId}
				summaries={summaries}
				total={total}
				totalPages={totalPages}
			/>

			<SummaryDetailDrawer
				botReady={botReady}
				chatTitle={selectedSummary ? chatTitles.get(selectedSummary.chatId) ?? "未知群组" : "未知群组"}
				onClose={() => setDetailOpen(false)}
				onOpenContext={() => setContextOpen(true)}
				onRerunSummary={rerunSummary}
				onRetryDelivery={retryDelivery}
				open={detailOpen && Boolean(selectedSummary)}
				selectedChat={selectedChat}
				selectedSummary={selectedSummary}
			/>

			<SummaryContextModal
				loading={contextLoading}
				onClose={() => setContextOpen(false)}
				open={contextOpen}
				preview={contextPreview}
			/>
		</DashboardPage>
	);
}

function asMessage(err: unknown) {
	if (err instanceof Error) {
		return err.message;
	}
	return String(err);
}
