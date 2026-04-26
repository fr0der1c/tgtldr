export type AuthStep = "idle" | "code" | "password" | "done";

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
  summaryParallelism: number;
  defaultTimezone: string;
  botEnabled: boolean;
  botToken?: string;
  botTargetChatId: string;
};

export type PendingAuth = {
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
  lastConnectedAt: string;
};

export type Bootstrap = {
  settingsConfigured: boolean;
  passwordConfigured: boolean;
  authenticated: boolean;
  telegramAuthorized: boolean;
  enabledChatCount: number;
  botEnabled: boolean;
  settings: AppSettings;
  auth?: TelegramAuth;
  pendingAuth?: PendingAuth;
};

export type AuthStatus = {
  status: string;
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

export type Chat = {
  id: number;
  telegramChatId: number;
  title: string;
  username: string;
  chatType: string;
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
  chunks: SummaryContextChunk[];
  finalInputNotice: string;
  previewNotice: string;
};
