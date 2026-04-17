"use client";

import { startTransition, useDeferredValue, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { AppSelect } from "@/components/app-select";
import { Chat } from "@/lib/types";
import { DashboardPage, EmptyState, MetricCard, MetricRail, Surface } from "@/components/dashboard-page";
import { useToast } from "@/components/toast";
import { Button, Field, Input, StatusPill, Textarea } from "@/components/ui";

type ChatTypeFilter = "all" | Chat["chatType"];
type SwitchFilter = "all" | "yes" | "no";

export function ChatsPanel() {
  const [items, setItems] = useState<Chat[]>([]);
  const [savedItems, setSavedItems] = useState<Chat[]>([]);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [chatType, setChatType] = useState<ChatTypeFilter>("all");
  const [messageSaveFilter, setMessageSaveFilter] = useState<SwitchFilter>("all");
  const [summaryFilter, setSummaryFilter] = useState<SwitchFilter>("all");
  const deferredQuery = useDeferredValue(query);
  const toast = useToast();

  useEffect(() => {
    void load();
  }, []);

  async function load() {
    try {
      const chats = (await api.listChats()).map(normalizeChat);
      setItems(chats);
      setSavedItems(chats);
      setEditingId((current) =>
        current && chats.some((chat) => chat.id === current) ? current : null
      );
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  async function saveChat(chat: Chat) {
    try {
      await api.saveChat(chat);
      toast.showSuccess(`已保存「${chat.title}」的配置。`);
      await load();
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  async function startHistoryBackfill(chat: Chat, fromDate: string, toDate: string) {
    try {
      const task = await api.startHistoryBackfill(chat.id, fromDate, toDate);
      toast.watchHistoryBackfill(task);
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  function patchChat(chatID: number, patch: Partial<Chat>) {
    setItems((current) =>
      current.map((item) => (item.id === chatID ? { ...item, ...patch } : item))
    );
  }

  const filtered = useMemo(() => {
    return items.filter((chat) => {
      if (!chat.title.toLowerCase().includes(deferredQuery.toLowerCase())) {
        return false;
      }
      if (chatType !== "all" && chat.chatType !== chatType) {
        return false;
      }
      if (messageSaveFilter !== "all") {
        const expected = messageSaveFilter === "yes";
        if (chat.enabled !== expected) {
          return false;
        }
      }
      if (summaryFilter !== "all") {
        const expected = summaryFilter === "yes";
        if (chat.summaryEnabled !== expected) {
          return false;
        }
      }
      return true;
    });
  }, [chatType, deferredQuery, items, messageSaveFilter, summaryFilter]);

  const syncedCount = savedItems.length;
  const messageSaveCount = savedItems.filter((chat) => chat.enabled).length;
  const summaryEnabledCount = savedItems.filter((chat) => chat.summaryEnabled).length;

  return (
    <DashboardPage
      title="群组"
      description="在这里选择需要保存消息和生成摘要的群组，并调整每个群的摘要设置。"
    >
      <MetricRail>
        <MetricCard
          label="已同步群组"
          value={syncedCount}
          badge="最新"
          detail="当前 Telegram 账号下可管理的群组与超级群组。"
        />
        <MetricCard
          label="已启用消息保存"
          value={messageSaveCount}
          tone={messageSaveCount > 0 ? "good" : "neutral"}
          badge={messageSaveCount > 0 ? "运行中" : "未启用"}
          detail="启用后，系统会实时保存该群的消息。"
        />
        <MetricCard
          label="已启用 AI 总结"
          value={summaryEnabledCount}
          tone={summaryEnabledCount > 0 ? "good" : "neutral"}
          badge={summaryEnabledCount > 0 ? "已配置" : "未启用"}
          detail="只有启用 AI 总结的群组才会参与每日摘要。"
        />
      </MetricRail>

      <Surface
        title="群组列表"
        description="先查看每个群当前是否启用，再通过操作按钮调整摘要规则、补充群聊背景或回补历史消息。"
      >
        <div className="toolbar-grid">
          <Field label="搜索群组">
            <Input
              placeholder="按群名搜索"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </Field>
          <Field label="群类型">
            <AppSelect
              onChange={(value) => setChatType(value as ChatTypeFilter)}
              options={[
                { value: "all", label: "全部" },
                { value: "group", label: "群组" },
                { value: "supergroup", label: "超级群组" }
              ]}
              value={chatType}
            />
          </Field>
          <Field label="消息保存">
            <AppSelect
              onChange={(value) => setMessageSaveFilter(value as SwitchFilter)}
              options={[
                { value: "all", label: "全部" },
                { value: "yes", label: "已启用" },
                { value: "no", label: "未启用" }
              ]}
              value={messageSaveFilter}
            />
          </Field>
          <Field label="AI 总结">
            <AppSelect
              onChange={(value) => setSummaryFilter(value as SwitchFilter)}
              options={[
                { value: "all", label: "全部" },
                { value: "yes", label: "已启用" },
                { value: "no", label: "未启用" }
              ]}
              value={summaryFilter}
            />
          </Field>
        </div>

        {filtered.length === 0 ? (
          <EmptyState
            title="没有匹配的群组"
            description="调整筛选条件后再试一次。"
          />
        ) : (
          <div className="data-table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>群组名称</th>
                  <th>群类型</th>
                  <th>消息保存</th>
                  <th>AI 总结</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((chat) => (
                  <ChatTableRow
                    key={chat.id}
                    chat={chat}
                    editing={editingId === chat.id}
                    onBackfill={(fromDate, toDate) =>
                      startTransition(() => void startHistoryBackfill(chat, fromDate, toDate))
                    }
                    onPatch={(patch) => patchChat(chat.id, patch)}
                    onEdit={() =>
                      setEditingId((current) => (current === chat.id ? null : chat.id))
                    }
                    onSave={() => startTransition(() => void saveChat(chat))}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Surface>

    </DashboardPage>
  );
}

function asMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function normalizeChat(chat: Chat): Chat {
  return {
    ...chat,
    summaryContext: chat.summaryContext ?? "",
    filteredKeywords: Array.isArray(chat.filteredKeywords) ? chat.filteredKeywords : [],
    filteredSenders: Array.isArray(chat.filteredSenders) ? chat.filteredSenders : [],
    keepBotMessages: chat.keepBotMessages ?? true
  };
}

function joinLines(values: string[]) {
  return values.join("\n");
}

function splitLines(value: string) {
  return value
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean);
}

function ChatTableRow({
  chat,
  editing,
  onBackfill,
  onPatch,
  onEdit,
  onSave
}: {
  chat: Chat;
  editing: boolean;
  onBackfill: (fromDate: string, toDate: string) => void;
  onPatch: (patch: Partial<Chat>) => void;
  onEdit: () => void;
  onSave: () => void;
}) {
  const [historyMode, setHistoryMode] = useState<"7d" | "30d" | "custom">("30d");
  const [historyFromDate, setHistoryFromDate] = useState(localDateOffset(-29));
  const [historyToDate, setHistoryToDate] = useState(localDateInputValue());
  const [historyExpanded, setHistoryExpanded] = useState(false);
  const historyRange = resolveHistoryRange(historyMode, historyFromDate, historyToDate);
  const expanded = editing || historyExpanded;

  return (
    <>
      <tr className={expanded ? "data-row active" : "data-row"}>
        <td>
          <div className="data-row-title">
            <strong>{chat.title}</strong>
            <span>{chat.username ? `@${chat.username}` : "无公开用户名"}</span>
          </div>
        </td>
        <td>{chat.chatType === "supergroup" ? "超级群组" : "群组"}</td>
        <td>
          <StatusPill tone={chat.enabled ? "good" : "neutral"}>
            {chat.enabled ? "已启用" : "未启用"}
          </StatusPill>
        </td>
        <td>
          <StatusPill tone={chat.summaryEnabled ? "good" : "neutral"}>
            {chat.summaryEnabled ? "已启用" : "未启用"}
          </StatusPill>
        </td>
        <td>
          <div className="table-row-actions">
            <Button
              aria-label={editing ? "收起编辑" : "编辑群组配置"}
              className={editing ? "table-icon-button active" : "table-icon-button"}
              onClick={onEdit}
              title={editing ? "收起编辑" : "编辑群组配置"}
              type="button"
              variant="ghost"
            >
              <EditIcon />
            </Button>
            <Button
              aria-label={historyExpanded ? "收起历史消息回补" : "加载历史消息"}
              className={historyExpanded ? "table-icon-button active" : "table-icon-button"}
              onClick={() => setHistoryExpanded((current) => !current)}
              title={historyExpanded ? "收起历史消息回补" : "加载历史消息"}
              type="button"
              variant="ghost"
            >
              <HistoryIcon />
            </Button>
          </div>
        </td>
      </tr>

      {expanded ? (
        <tr className="data-row-detail">
          <td colSpan={5}>
            <div className="table-editor">
              {editing ? (
                <>
                  <div className="form-grid table-editor-primary-grid">
                    <Field label="消息保存">
                      <AppSelect
                        onChange={(value) => onPatch({ enabled: value === "yes" })}
                        options={[
                          { value: "yes", label: "启用" },
                          { value: "no", label: "停用" }
                        ]}
                        value={chat.enabled ? "yes" : "no"}
                      />
                    </Field>

                    <Field label="AI 总结">
                      <AppSelect
                        onChange={(value) =>
                          onPatch({ summaryEnabled: value === "yes" })
                        }
                        options={[
                          { value: "yes", label: "启用" },
                          { value: "no", label: "停用" }
                        ]}
                        value={chat.summaryEnabled ? "yes" : "no"}
                      />
                    </Field>
                  </div>

                  {chat.summaryEnabled ? (
                    <>
                      <div className="form-grid">
                        <Field label="AI 总结交付方式">
                          <AppSelect
                            onChange={(value) =>
                              onPatch({
                                deliveryMode: value as Chat["deliveryMode"]
                              })
                            }
                            options={[
                              { value: "dashboard", label: "仅在网页端查看" },
                              { value: "bot", label: "通过 Bot 推送" }
                            ]}
                            value={chat.deliveryMode}
                          />
                        </Field>

                        <Field label="摘要时间">
                          <Input
                            value={chat.summaryTimeLocal}
                            onChange={(event) =>
                              onPatch({ summaryTimeLocal: event.target.value })
                            }
                          />
                        </Field>

                        <Field label="模型 override">
                          <Input
                            placeholder="例如 gpt-4.1-mini"
                            value={chat.modelOverride}
                            onChange={(event) =>
                              onPatch({ modelOverride: event.target.value })
                            }
                          />
                        </Field>

                        <Field label="保留机器人消息">
                          <AppSelect
                            onChange={(value) =>
                              onPatch({ keepBotMessages: value === "yes" })
                            }
                            options={[
                              { value: "yes", label: "保留" },
                              { value: "no", label: "过滤" }
                            ]}
                            value={chat.keepBotMessages ? "yes" : "no"}
                          />
                        </Field>
                      </div>
                      <p className="table-editor-note">
                        模型 override 留空时会跟随系统默认模型。
                      </p>

                      <div className="form-grid">
                        <Field
                          label="过滤发言人"
                          hint="每行一个，支持昵称或 @username，精确匹配。"
                        >
                          <Textarea
                            rows={5}
                            placeholder={"验证机器人\n@verify_bot"}
                            value={joinLines(chat.filteredSenders)}
                            onChange={(event) =>
                              onPatch({ filteredSenders: splitLines(event.target.value) })
                            }
                          />
                        </Field>

                        <Field
                          label="过滤关键词"
                          hint="每行一个，按包含关系过滤消息内容。"
                        >
                          <Textarea
                            rows={5}
                            placeholder={"请完成入群验证\n验证已过期"}
                            value={joinLines(chat.filteredKeywords)}
                            onChange={(event) =>
                              onPatch({ filteredKeywords: splitLines(event.target.value) })
                            }
                          />
                        </Field>
                      </div>

                      <Field
                        label="群聊背景"
                        hint="补充群里常见黑话、术语、长期背景或默认语境，帮助模型正确理解讨论内容。"
                      >
                        <Textarea
                          rows={6}
                          placeholder="例如：这个群主要讨论二级市场和链上项目；ATL 指 All Time Low；喊单通常是半开玩笑表达。"
                          value={chat.summaryContext}
                          onChange={(event) =>
                            onPatch({ summaryContext: event.target.value })
                          }
                        />
                      </Field>

                      <Field label="摘要额外要求">
                        <Textarea
                          rows={8}
                          placeholder="告诉模型你希望如何总结这个群的消息，例如保留决策、行动项和重要链接。"
                          value={chat.summaryPrompt}
                          onChange={(event) =>
                            onPatch({ summaryPrompt: event.target.value })
                          }
                        />
                      </Field>
                    </>
                  ) : null}

                  <div className="editor-footer">
                    <p className="muted">
                      当前群类型：{chat.chatType === "supergroup" ? "超级群组" : "群组"}
                    </p>
                    <Button onClick={onSave} type="button">
                      保存该群配置
                    </Button>
                  </div>
                </>
              ) : null}

              {historyExpanded ? (
                <div className="history-backfill-panel">
                  <div className="history-backfill-head">
                    <strong>加载历史消息</strong>
                    <p>回补后，这个群较早时间段的消息也可以用于生成摘要。</p>
                  </div>
                  <div className="form-grid">
                    <Field label="时间范围">
                      <AppSelect
                        onChange={(value) => setHistoryMode(value as "7d" | "30d" | "custom")}
                        options={[
                          { value: "7d", label: "最近 7 天" },
                          { value: "30d", label: "最近 30 天" },
                          { value: "custom", label: "自定义日期范围" }
                        ]}
                        value={historyMode}
                      />
                    </Field>
                    {historyMode === "custom" ? (
                      <>
                        <Field label="开始日期">
                          <Input
                            type="date"
                            value={historyFromDate}
                            onChange={(event) => setHistoryFromDate(event.target.value)}
                          />
                        </Field>
                        <Field label="结束日期">
                          <Input
                            type="date"
                            value={historyToDate}
                            onChange={(event) => setHistoryToDate(event.target.value)}
                          />
                        </Field>
                      </>
                    ) : (
                      <div className="history-backfill-range muted">
                        将回补 {historyRange.fromDate} 到 {historyRange.toDate} 的消息。
                      </div>
                    )}
                  </div>
                  <div className="history-backfill-actions">
                    <Button
                      disabled={!historyRange.fromDate || !historyRange.toDate}
                      onClick={() => onBackfill(historyRange.fromDate, historyRange.toDate)}
                      type="button"
                      variant="secondary"
                    >
                      加载历史消息
                    </Button>
                  </div>
                </div>
              ) : null}
            </div>
          </td>
        </tr>
      ) : null}
    </>
  );
}

function resolveHistoryRange(
  mode: "7d" | "30d" | "custom",
  customFromDate: string,
  customToDate: string
) {
  if (mode === "custom") {
    return { fromDate: customFromDate, toDate: customToDate };
  }
  if (mode === "7d") {
    return { fromDate: localDateOffset(-6), toDate: localDateInputValue() };
  }
  return { fromDate: localDateOffset(-29), toDate: localDateInputValue() };
}

function localDateOffset(offsetDays: number) {
  const now = new Date();
  now.setDate(now.getDate() + offsetDays);
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}

function localDateInputValue() {
  const now = new Date();
  const local = new Date(now.getTime() - now.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}

function EditIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <path
        d="M4 20h4l10-10-4-4L4 16v4Z"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
      <path
        d="M13.5 6.5l4 4"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
    </svg>
  );
}

function HistoryIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="16" viewBox="0 0 24 24" width="16">
      <path
        d="M3 12a9 9 0 1 0 3-6.708"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
      <path
        d="M3 4v4h4"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
      <path
        d="M12 7.5v5l3 2"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />
    </svg>
  );
}
