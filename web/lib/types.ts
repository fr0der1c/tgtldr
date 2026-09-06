export type AuthStep = "idle" | "code" | "password" | "done";
export type Language = "zh-CN" | "en";
export type BotSummaryDeliveryMode = "per_chat" | "daily_digest";

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
  openAIContextWindowMode: "auto" | "manual";
  openAIContextWindowTokens: number;
  openAIRequestMode: "stream" | "non_stream";
  summaryParallelism: number;
  summaryRetryLimit: number;
  summaryRetryBackoffBaseMinutes: number;
  summaryRetryBackoffMultiplier: number;
  defaultTimezone: string;
  language: Language;
  autoDownloadAttachments: boolean;
  botSummaryDeliveryMode: BotSummaryDeliveryMode;
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
  kind: "photo" | "video" | "audio" | "voice" | "document" | "sticker";
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
  timezone?: string;
};

export type GlobalChatMessageSearchItem = ChatMessageSearchItem & {
  chatId: number;
  chatTitle: string;
};

export type GlobalChatMessageSearchResponse = Omit<ChatMessageSearchResponse, "items"> & {
  items: GlobalChatMessageSearchItem[];
  timezone: string;
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
  requestedModel?: string;
  returnedModel?: string;
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
  botSummaryDeliveryMode: BotSummaryDeliveryMode;
  dailyDigestId?: number;
  dailyDigestIncluded?: boolean;
  dailyDigestOmissionReason?: string;
  dailyDigestStatus?: Summary["status"];
  dailyDigestDeliveredAt?: string;
  dailyDigestDeliveryError?: string;
  dailyDigestDeliverySkippedReason?: string;
  dailyDigestDeliverySuppressed?: boolean;
  matchSnippet?: string;
  matchedFields?: string[];
};

export type SummarySearchFilters = {
  q?: string;
  chatId?: string;
  status?: "all" | "processing" | Summary["status"];
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

export type CatchUpChat = {
  chatId: number;
  chatTitle: string;
  sourceSummaryCount: number;
};

export type CatchUpSource = {
  summaryId: number;
  chatId: number;
  chatTitle: string;
  summaryDate: string;
};

export type CatchUp = {
  id: number;
  fromDate: string;
  toDate: string;
  status: Summary["status"];
  content: string;
  model: string;
  chatCount: number;
  sourceSummaryCount: number;
  chunkCount: number;
  executionMode: "" | "single" | "chunked" | "fallback_chunked";
  estimatedInputTokens: number;
  contextWindowTokens: number;
  fallbackReason: string;
  deliveryRequested: boolean;
  deliveredAt?: string;
  deliveryError: string;
  errorMessage: string;
  generatedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
  chats?: CatchUpChat[];
  sources?: CatchUpSource[];
};

export type CatchUpListResponse = {
  items: CatchUp[];
  total: number;
  page: number;
  pageSize: number;
};

export type DailyDigestSource = {
  summaryId: number;
  chatId: number;
  chatTitle: string;
  summaryStatus: Summary["status"];
  sourceMessageCount: number;
  included: boolean;
  omissionReason: "" | "no_messages" | "generation_failed" | "empty_content";
};

export type DailyDigest = {
  id: number;
  summaryDate: string;
  status: Summary["status"];
  content: string;
  model: string;
  participantCount: number;
  sourceSummaryCount: number;
  emptyChatCount: number;
  omittedChatCount: number;
  chunkCount: number;
  executionMode: "" | "single" | "chunked" | "fallback_chunked" | "passthrough" | "no_content";
  estimatedInputTokens: number;
  contextWindowTokens: number;
  fallbackReason: string;
  deliverySkippedReason: string;
  deliverySuppressed: boolean;
  deliveredAt?: string;
  deliveryError: string;
  errorMessage: string;
  retryCount: number;
  nextRetryAt?: string;
  generatedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
  sources?: DailyDigestSource[];
};

export type DailyDigestListResponse = {
  items: DailyDigest[];
  total: number;
  page: number;
  pageSize: number;
};
