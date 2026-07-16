export type AuthStep = "idle" | "code" | "password" | "done";
export type Language = "zh-CN" | "en";

export type AppSettings = {
  id: number;
  telegramApiId: number;
  telegramApiHash?: string;
  openAIBaseUrl: string;
  openAIApiKey?: string;
  openAIModel: string;
  openAITemperature: number;
  openAIOutputMode: "auto" | "manual";
  openAIMaxOutputTokens: number;
  openAIRequestMode: "stream" | "non_stream";
  summaryParallelism: number;
  summaryRetryLimit: number;
  summaryRetryBackoffBaseMinutes: number;
  summaryRetryBackoffMultiplier: number;
  defaultTimezone: string;
  language: Language;
  autoDownloadAttachments: boolean;
  botEnabled: boolean;
  botToken?: string;
  botTargetChatId: string;
};

export type PendingAuth = {
  accountId: number;
  step: AuthStep;
  phoneNumber: string;
  deadline: string;
};

export type TelegramAuth = {
  id: number;
  phoneNumber: string;
  telegramUserId: number;
  telegramName: string;
  telegramHandle: string;
  status: string;
  usedByChatCount: number;
  lastConnectedAt: string;
};

export type TelegramAccountChat = {
  accountId: number;
  chatId: number;
  telegramAccessHash: number;
  accountName: string;
  accountHandle: string;
  accountStatus: string;
};

export type Bootstrap = {
  settingsConfigured: boolean;
  passwordConfigured: boolean;
  authenticated: boolean;
  telegramAuthorized: boolean;
  enabledChatCount: number;
  botEnabled: boolean;
  language: Language;
  settings: AppSettings;
  auth?: TelegramAuth;
  telegramAccounts: TelegramAuth[];
  authorizedAccountCount: number;
  pendingAuth?: PendingAuth;
};

export type AuthStatus = {
  status: string;
};

export type OpenAITestResult = {
  ok: boolean;
  model: string;
};

export type BotTargetChatCandidate = {
  chatId: string;
  chatType: string;
  title?: string;
  username?: string;
};

export type BotTargetChatResolveResult = {
  candidates: BotTargetChatCandidate[];
};

export type DeliveryMode = "dashboard" | "bot";

export type ChatMessageActivity = {
  date: string;
  messageCount: number;
};

export type ChatMessageReply = {
  telegramMessageId: number;
  found: boolean;
  senderName: string;
  textContent: string;
  caption: string;
  messageType: string;
  mediaKind: string;
};

export type ChatMessage = {
  id: number;
  telegramMessageId: number;
  senderName: string;
  senderUsername: string;
  senderIsBot: boolean;
  textContent: string;
  caption: string;
  messageType: string;
  mediaKind: string;
  messageTime: string;
  reply?: ChatMessageReply;
  senderAvatarUrl?: string;
  media?: ChatMessageMedia;
};

export type ChatMessageMedia = {
  id: number;
  kind: "photo" | "video" | "audio" | "voice" | "document";
  mimeType: string;
  fileName: string;
  size: number;
  status: "manual" | "pending" | "downloading" | "succeeded" | "skipped_oversize" | "failed";
  contentUrl?: string;
  error?: string;
  canDownload: boolean;
  canRetry: boolean;
};

export type ChatMessageListResponse = {
  chat: {
    id: number;
    title: string;
    username: string;
    enabled: boolean;
    avatarUrl?: string;
  };
  date: string;
  timezone: string;
  total: number;
  messages: ChatMessage[];
  messageActivity: ChatMessageActivity[];
  previousDate?: string;
  nextDate?: string;
  hasMoreBefore: boolean;
  beforeCursor?: string;
  focusedMessageId?: number;
  hasMessageFilters: boolean;
  filtersApplied: boolean;
};

export type ChatMessageSearchItem = ChatMessage & {
  localDate: string;
  matchSnippet: string;
  matchedFields: string[];
};

export type ChatMessageSearchResponse = {
  items: ChatMessageSearchItem[];
  total: number;
  page: number;
  pageSize: number;
};

export type Chat = {
  id: number;
  telegramChatId: number;
  title: string;
  username: string;
  chatType: string;
  collectorAccountId: number;
  availableAccounts: TelegramAccountChat[];
  enabled: boolean;
  summaryEnabled: boolean;
  summaryContext: string;
  summaryPrompt: string;
  summaryTimeLocal: string;
  deliveryMode: DeliveryMode;
  modelOverride: string;
  keepBotMessages: boolean;
  filteredSenders: string[];
  filteredKeywords: string[];
  messageActivity?: ChatMessageActivity[];
  avatarUrl?: string;
};

export type HistoryBackfillTask = {
  id: string;
  chatId: number;
  chatTitle: string;
  fromDate: string;
  toDate: string;
  status: "pending" | "running" | "succeeded" | "failed";
  importedCount: number;
  errorMessage: string;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
};

export type Summary = {
  id: number;
  chatId: number;
  summaryDate: string;
  status: "pending" | "running" | "succeeded" | "failed";
  content: string;
  model: string;
  sourceMessageCount: number;
  chunkCount: number;
  generatedAt: string;
  deliveredAt?: string;
  deliveryError: string;
  errorMessage: string;
  errorContext: string;
  errorSystemPrompt: string;
  errorUserPrompt: string;
  retryCount: number;
  nextRetryAt?: string;
  matchSnippet?: string;
  matchedFields?: string[];
};

export type SummarySearchFilters = {
  q?: string;
  chatId?: string;
  status?: "all" | Summary["status"];
  delivery?: "all" | "sent" | "pending" | "failed" | "disabled";
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  pageSize?: number;
};

export type SummaryListResponse = {
  items: Summary[];
  total: number;
  page: number;
  pageSize: number;
};

export type SummaryStats = {
  total: number;
  successCount: number;
  processingCount: number;
  failedCount: number;
};

export type SummaryContextChunk = {
  index: number;
  messageCount: number;
  content: string;
};

export type SummaryContextPreview = {
  summaryId: number;
  chatId: number;
  summaryDate: string;
  model: string;
  systemPrompt: string;
  finalPrompt: string;
  messageCount: number;
  chunkCount: number;
  chunks: SummaryContextChunk[] | null;
  finalInputNotice: string;
  previewNotice: string;
};
