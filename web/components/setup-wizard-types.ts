import { AppSettings, Bootstrap, PendingAuth } from "@/lib/types";

export type SetupStep = "config" | "login" | "bot";
export type LoginStage = "phone" | "code" | "password" | "success";

export type StepDescriptor = {
  key: SetupStep;
  label: string;
  eyebrow: string;
};

export type WizardState = {
  bootstrap: Bootstrap | null;
  settings: AppSettings;
  currentStep: SetupStep;
  countryCode: string;
  phoneNumber: string;
  code: string;
  password: string;
  pendingAuth: PendingAuth | null;
  loginStageOverride: LoginStage | null;
  discoveredChats: number;
  error: string;
  notice: string;
};

export const setupSteps: StepDescriptor[] = [
  { key: "config", label: "基础配置", eyebrow: "第 1 步" },
  { key: "login", label: "登录", eyebrow: "第 2 步" },
  { key: "bot", label: "Bot 推送", eyebrow: "第 3 步" }
];

export const knownOpenAIModels = [
  "gpt-5.4",
  "gpt-5.4-mini",
  "gpt-5.2",
  "gpt-4.1",
  "gpt-4.1-mini"
] as const;

export const emptySettings: AppSettings = {
  id: 0,
  telegramApiId: 0,
  telegramApiHash: "",
  openAIBaseUrl: "https://api.openai.com/v1",
  openAIApiKey: "",
  openAIModel: "gpt-4.1-mini",
  openAITemperature: 0.2,
  openAIOutputMode: "auto",
  openAIMaxOutputTokens: 2000,
  summaryParallelism: 2,
  defaultTimezone: "Asia/Shanghai",
  botEnabled: false,
  botToken: "",
  botTargetChatId: ""
};
