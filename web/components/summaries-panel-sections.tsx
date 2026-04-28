"use client";

import { startTransition } from "react";
import { AppSelect } from "@/components/app-select";
import { SearchSelect } from "@/components/search-select";
import {
	EmptyState,
	Surface,
} from "@/components/dashboard-page";
import { Button, Field, Input, StatusPill } from "@/components/ui";
import { TextHighlight } from "@/components/text-highlight";
import { Chat, Summary } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

export type SummaryFilter = "all" | Summary["status"];
export type DeliveryFilter = "all" | "sent" | "pending" | "failed" | "disabled";
export type DeliveryTone = "neutral" | "good" | "warn" | "bad";
export type DeliveryState = {
	label: string;
	tone: DeliveryTone;
	detail?: string;
	retryable: boolean;
};

type SummaryListSectionProps = {
	allChats: Chat[];
	botReady: boolean;
	chatFilter: string;
	chats: Chat[];
	dateFrom: string;
	dateTo: string;
	deliveryFilter: DeliveryFilter;
	filter: SummaryFilter;
	loadSummaryDate: string;
	manualEditorOpen: boolean;
	page: number;
	query: string;
	searching: boolean;
	selectedChatId: string;
	selectedSummaryId: number | null;
	searchTerms: string[];
	summaries: Summary[];
	total: number;
	totalPages: number;
	onChatFilterChange: (value: string) => void;
	onDateFromChange: (value: string) => void;
	onDateToChange: (value: string) => void;
	onDeliveryFilterChange: (value: DeliveryFilter) => void;
	onFilterChange: (value: SummaryFilter) => void;
	onLoadSummaryDateChange: (value: string) => void;
	onManualEditorToggle: () => void;
	onManualRun: () => Promise<void>;
	onPageChange: (value: number) => void;
	onQueryChange: (value: string) => void;
	onSelectedChatChange: (value: string) => void;
	onSelectSummary: (summaryId: number) => void;
	chatTitles: Map<number, string>;
};

export function SummaryListSection(props: SummaryListSectionProps) {
	const { language } = useI18n();
	const {
		allChats,
		botReady,
		chatFilter,
		chats,
		dateFrom,
		dateTo,
		deliveryFilter,
		filter,
		loadSummaryDate,
		manualEditorOpen,
		page,
		query,
		searching,
		selectedChatId,
		selectedSummaryId,
		searchTerms,
		summaries,
		total,
		totalPages,
		onChatFilterChange,
		onDateFromChange,
		onDateToChange,
		onDeliveryFilterChange,
		onFilterChange,
		onLoadSummaryDateChange,
		onManualEditorToggle,
		onManualRun,
		onPageChange,
		onQueryChange,
		onSelectedChatChange,
		onSelectSummary,
		chatTitles,
	} = props

	return (
		<Surface
			description="在这里搜索和筛选摘要记录；点开某条摘要后，会从右侧展开完整正文。"
			title="摘要记录"
		>
			<div className="toolbar-grid summary-search-grid">
				<Field label="搜索摘要关键词">
					<Input onChange={(event) => onQueryChange(event.target.value)} placeholder="搜索摘要关键词" value={query} />
				</Field>
				<Field label="群组">
					<SearchSelect
						emptyText="没有匹配的群组"
						onChange={onChatFilterChange}
						options={[
							{ value: "all", label: "全部群组" },
							...allChats.map((chat) => ({
								value: String(chat.id),
								label: chat.title,
							})),
						]}
						placeholder="全部群组"
						searchPlaceholder="搜索群组"
						value={chatFilter}
					/>
				</Field>
				<Field label="生成状态">
					<AppSelect
						onChange={(value) => onFilterChange(value as SummaryFilter)}
						options={[
							{ value: "all", label: "全部状态" },
							{ value: "succeeded", label: "成功" },
							{ value: "running", label: "运行中" },
							{ value: "pending", label: "等待中" },
							{ value: "failed", label: "失败" },
						]}
						value={filter}
					/>
				</Field>
				<Field label="发送状态">
					<AppSelect
						onChange={(value) => onDeliveryFilterChange(value as DeliveryFilter)}
						options={[
							{ value: "all", label: "全部" },
							{ value: "sent", label: "已发送" },
							{ value: "pending", label: "待发送" },
							{ value: "failed", label: "发送失败" },
							{ value: "disabled", label: "不发送" },
						]}
						value={deliveryFilter}
					/>
				</Field>
				<Field label="开始日期">
					<Input onChange={(event) => onDateFromChange(event.target.value)} type="date" value={dateFrom} />
				</Field>
				<Field label="结束日期">
					<Input onChange={(event) => onDateToChange(event.target.value)} type="date" value={dateTo} />
				</Field>
			</div>

			<div className="summary-toolbar">
				<div className="summary-toolbar-meta">
					<span>{language === "en" ? `Page ${page} / ${totalPages}` : `第 ${page} / ${totalPages} 页`}</span>
					<span>{language === "en" ? `${total} summaries` : `共 ${total} 条摘要`}</span>
				</div>
				<Button className="summary-toolbar-button" onClick={onManualEditorToggle} type="button" variant="secondary">
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
						<EmptyState description="只有已启用 AI 总结的群组才会出现在这里。" title="还没有可补跑的群组" />
					) : (
						<>
							<div className="form-grid">
								<Field label="群组">
									<SearchSelect
										emptyText="没有匹配的群组"
										onChange={onSelectedChatChange}
										options={chats.map((chat) => ({
											value: String(chat.id),
											label: chat.title,
										}))}
										placeholder="选择群组"
										searchPlaceholder="搜索群组"
										value={selectedChatId}
									/>
								</Field>
								<Field label="日期">
									<Input onChange={(event) => onLoadSummaryDateChange(event.target.value)} type="date" value={loadSummaryDate} />
								</Field>
							</div>
							<div className="summary-manual-actions">
								<Button onClick={() => startTransition(() => void onManualRun())} type="button">
									立即生成
								</Button>
							</div>
						</>
					)}
				</div>
			) : null}

			{summaries.length === 0 ? (
				<EmptyState
					description={searching ? "换个关键词或调整筛选条件后再试一次。" : "先展开手动补跑触发一次摘要，或者等待定时任务执行。"}
					title={searching ? "没有匹配的摘要" : "还没有摘要记录"}
				/>
			) : (
				<>
					<div className="entity-list">
						{summaries.map((item) => {
							const delivery = deliveryState(item, allChats.find((chat) => chat.id === item.chatId) ?? null, botReady)
							return (
								<button
									key={item.id}
									className={`entity-row ${item.id === selectedSummaryId ? "active" : ""}`}
									onClick={() => onSelectSummary(item.id)}
									type="button"
								>
									<div className="entity-row-main">
										<strong>{chatTitles.get(item.chatId) ?? "未知群组"}</strong>
										<p>
											{item.summaryDate} · {item.model || "未记录模型"}
										</p>
										{searching && item.matchSnippet ? (
											<p className="entity-row-snippet">
												<TextHighlight terms={searchTerms} text={item.matchSnippet} />
											</p>
										) : null}
									</div>
									<div className="entity-row-meta">
										<StatusPill tone={statusTone(item.status)}>{statusText(item.status)}</StatusPill>
										<StatusPill className={delivery.detail ? "status-pill-hoverable" : undefined} title={delivery.detail} tone={delivery.tone}>
											{delivery.label}
										</StatusPill>
									</div>
								</button>
							)
						})}
					</div>

					<div className="summary-pagination">
						<Button disabled={page <= 1} onClick={() => onPageChange(Math.max(1, page - 1))} type="button" variant="secondary">
							上一页
						</Button>
						<span>
							{language === "en" ? `Page ${page} of ${totalPages}` : `第 ${page} 页，共 ${totalPages} 页`}
						</span>
						<Button
							disabled={page >= totalPages}
							onClick={() => onPageChange(Math.min(totalPages, page + 1))}
							type="button"
							variant="secondary"
						>
							下一页
						</Button>
					</div>
				</>
			)}
		</Surface>
	)
}

export function statusTone(status: Summary["status"]) {
	if (status === "succeeded") return "good"
	if (status === "failed") return "bad"
	if (status === "running") return "warn"
	return "neutral"
}

export function statusText(status: Summary["status"]) {
	if (status === "succeeded") return "成功"
	if (status === "failed") return "失败"
	if (status === "running") return "运行中"
	return "等待中"
}

export function deliveryState(summary: Summary, chat: Chat | null, botReady: boolean): DeliveryState {
	if (!chat || chat.deliveryMode !== "bot") {
		return { label: "不发送", tone: "neutral", detail: "当前群组设置为不通过 Bot 推送。", retryable: false }
	}
	if (!botReady) {
		return { label: "待发送", tone: "warn", detail: "Bot 配置尚未完成，当前无法发送。", retryable: false }
	}
	if (summary.status !== "succeeded") {
		return { label: "未发送", tone: "neutral", detail: "摘要尚未生成成功，当前不会执行发送。", retryable: false }
	}
	if (summary.deliveredAt) {
		return { label: "已发送", tone: "good", detail: `已发送于 ${summary.deliveredAt}`, retryable: false }
	}
	if (summary.deliveryError) {
		return { label: "发送失败", tone: "bad", detail: summary.deliveryError, retryable: true }
	}
	return { label: "待发送", tone: "warn", detail: "摘要已生成，等待自动发送或手动重试。", retryable: true }
}

export function localDateInputValue() {
	const now = new Date()
	const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000)
	return local.toISOString().slice(0, 10)
}
