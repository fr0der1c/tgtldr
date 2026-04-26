import {
  AppSettings,
  AuthStatus,
  Bootstrap,
  BotTargetChatResolveResult,
  Chat,
  HistoryBackfillTask,
  SummaryListResponse,
  SummarySearchFilters,
  Summary,
  SummaryStats,
  SummaryContextPreview,
} from "@/lib/types";

type ErrorPayload = {
  error?: string;
  code?: string;
  retryAfterSeconds?: number;
};

export class APIError extends Error {
  status: number;
  code?: string;
  retryAfterSeconds?: number;

  constructor(message: string, status: number, payload?: ErrorPayload) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = payload?.code;
    this.retryAfterSeconds = payload?.retryAfterSeconds;
  }
}

function normalizeList<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

type RequestOptions = RequestInit & {
  skipUnauthorizedRedirect?: boolean;
};

async function request<T>(path: string, init?: RequestOptions): Promise<T> {
  const response = await fetch(`${resolveAPIBaseURL()}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    cache: "no-store",
  });

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    let payload: ErrorPayload | undefined;
    try {
      payload = (await response.json()) as ErrorPayload;
      if (typeof payload.error === "string") {
        message = payload.error;
      }
    } catch {
      // ignore
    }
    if (
      response.status === 401 &&
      typeof window !== "undefined" &&
      !init?.skipUnauthorizedRedirect &&
      !window.location.pathname.startsWith("/login")
    ) {
      window.location.href = "/login";
    }
    throw new APIError(message, response.status, payload);
  }

  return response.json() as Promise<T>;
}

function resolveAPIBaseURL() {
  if (typeof window !== "undefined") {
    return `${window.location.protocol}//${window.location.hostname}:8080`;
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";
}

function buildQuery(params: Record<string, string | number | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") {
      continue;
    }
    search.set(key, String(value));
  }
  const encoded = search.toString();
  if (!encoded) {
    return "";
  }
  return `?${encoded}`;
}

export const api = {
  bootstrap: () => request<Bootstrap>("/api/bootstrap"),
  login: (password: string) =>
    request<AuthStatus>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ password }),
      skipUnauthorizedRedirect: true,
    }),
  logout: () =>
    request<AuthStatus>("/api/auth/logout", {
      method: "POST",
      skipUnauthorizedRedirect: true,
    }),
  setupPassword: (password: string) =>
    request<AuthStatus>("/api/auth/setup-password", {
      method: "POST",
      body: JSON.stringify({ password }),
      skipUnauthorizedRedirect: true,
    }),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<AuthStatus>("/api/auth/change-password", {
      method: "POST",
      body: JSON.stringify({ currentPassword, newPassword }),
    }),
  settings: () => request<AppSettings>("/api/settings"),
  saveSettings: (payload: AppSettings) =>
    request<AppSettings>("/api/settings", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  resolveBotTargetChat: (botToken?: string) =>
    request<BotTargetChatResolveResult>("/api/bot/target-chat/resolve", {
      method: "POST",
      body: JSON.stringify({ botToken: botToken?.trim() || "" }),
    }),
  startAuth: (phoneNumber: string) =>
    request("/api/telegram/auth/start", {
      method: "POST",
      body: JSON.stringify({ phoneNumber }),
    }),
  verifyCode: (code: string) =>
    request("/api/telegram/auth/code", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),
  verifyPassword: (password: string) =>
    request("/api/telegram/auth/password", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),
  syncChats: async () =>
    normalizeList(
      await request<Chat[] | null>("/api/telegram/chats/sync", {
        method: "POST",
      }),
    ),
  listChats: async () =>
    normalizeList(await request<Chat[] | null>("/api/chats")),
  saveChat: (chat: Chat) =>
    request<Chat>(`/api/chats/${chat.id}`, {
      method: "PUT",
      body: JSON.stringify({
        enabled: chat.enabled,
        summaryEnabled: chat.summaryEnabled,
        summaryContext: chat.summaryContext,
        summaryPrompt: chat.summaryPrompt,
        summaryTimeLocal: chat.summaryTimeLocal,
        deliveryMode: chat.deliveryMode,
        modelOverride: chat.modelOverride,
        keepBotMessages: chat.keepBotMessages,
        filteredSenders: chat.filteredSenders,
        filteredKeywords: chat.filteredKeywords,
      }),
    }),
  startHistoryBackfill: (chatId: number, fromDate: string, toDate: string) =>
    request<HistoryBackfillTask>("/api/history-backfills", {
      method: "POST",
      body: JSON.stringify({ chatId, fromDate, toDate }),
    }),
  getHistoryBackfill: (taskId: string) =>
    request<HistoryBackfillTask>(`/api/history-backfills/${taskId}`),
  summaryStats: () => request<SummaryStats>("/api/summaries/stats"),
  listSummaries: (filters?: SummarySearchFilters) =>
    request<SummaryListResponse>(
      `/api/summaries${buildQuery({
        q: filters?.q?.trim() || undefined,
        chatId: filters?.chatId && filters.chatId !== "all" ? filters.chatId : undefined,
        status: filters?.status && filters.status !== "all" ? filters.status : undefined,
        delivery: filters?.delivery && filters.delivery !== "all" ? filters.delivery : undefined,
        dateFrom: filters?.dateFrom || undefined,
        dateTo: filters?.dateTo || undefined,
        page: filters?.page,
        pageSize: filters?.pageSize,
      })}`,
    ),
  summaryContextPreview: (summaryId: number) =>
    request<SummaryContextPreview>(
      `/api/summaries/context-preview?id=${summaryId}`,
    ),
  retrySummaryDelivery: (summaryId: number) =>
    request(`/api/summaries/${summaryId}/retry-delivery`, {
      method: "POST",
    }),
  runSummary: (chatId: number, date: string) =>
    request("/api/summaries/run", {
      method: "POST",
      body: JSON.stringify({ chatId, date }),
    }),
};
