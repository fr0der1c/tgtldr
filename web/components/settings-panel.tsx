"use client";

import { CSSProperties, startTransition, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { APIError, api } from "@/lib/api";
import { AppSelect } from "@/components/app-select";
import { SearchSelect } from "@/components/search-select";
import {
  AppSettings,
  Bootstrap,
  BotTargetChatCandidate,
  PendingAuth,
  TelegramAuth,
} from "@/lib/types";
import {
  describeBotChatCandidate,
  hasAvailableBotToken,
} from "@/lib/bot-target-chat";
import { notifyBootstrapRefresh } from "@/lib/bootstrap-sync";
import { DashboardPage, Surface } from "@/components/dashboard-page";
import { useToast } from "@/components/toast";
import { Button, Field, Input, StatusPill } from "@/components/ui";
import { listTimezoneOptions } from "@/lib/timezones";
import { normalizeLanguage, useI18n } from "@/lib/i18n";

type SecretPlaceholders = {
  botToken: string;
  openAIApiKey: string;
  telegramApiHash: string;
};

type AuthStage = "summary" | "phone" | "code" | "password";
type SettingsTab = "telegram" | "summary" | "preferences" | "security" | "bot";

const settingsTabs: Array<{ value: SettingsTab; label: string }> = [
  { value: "telegram", label: "Telegram" },
  { value: "summary", label: "摘要引擎" },
  { value: "preferences", label: "偏好设置" },
  { value: "security", label: "安全" },
  { value: "bot", label: "Bot 推送" },
];

export function SettingsPanel() {
  const router = useRouter();
  const { dict, setLanguage } = useI18n();
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);
  const [pendingAuth, setPendingAuth] = useState<PendingAuth | null>(null);
  const [secretPlaceholders, setSecretPlaceholders] =
    useState<SecretPlaceholders>({
      botToken: "",
      openAIApiKey: "",
      telegramApiHash: "",
    });
  const [countryCode, setCountryCode] = useState("+86");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [currentAccessPassword, setCurrentAccessPassword] = useState("");
  const [nextAccessPassword, setNextAccessPassword] = useState("");
  const [nextAccessPasswordConfirm, setNextAccessPasswordConfirm] =
    useState("");
  const [authEditorOpen, setAuthEditorOpen] = useState(false);
  const [reauthAccountId, setReauthAccountId] = useState<number | null>(null);
  const [authRetryUntil, setAuthRetryUntil] = useState<number | null>(null);
  const [authRetryNow, setAuthRetryNow] = useState(Date.now());
  const [botTargetChatCandidates, setBotTargetChatCandidates] = useState<
    BotTargetChatCandidate[]
  >([]);
  const [resolvingBotTargetChat, setResolvingBotTargetChat] = useState(false);
  const [savingBotTargetChat, setSavingBotTargetChat] = useState(false);
  const [testingOpenAI, setTestingOpenAI] = useState(false);
  const [botTargetTelegramAccountID, setBotTargetTelegramAccountID] = useState(0);
  const [activeTab, setActiveTab] = useState<SettingsTab>("telegram");
  const toast = useToast();
  const timezoneOptions = useMemo(() => listTimezoneOptions(), []);

  useEffect(() => {
    void load();
    setActiveTab(readSettingsTab());
  }, []);

  useEffect(() => {
    if (!authRetryUntil) {
      return;
    }
    const timer = window.setInterval(() => setAuthRetryNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [authRetryUntil]);

  async function load() {
    try {
      const [settingsData, bootstrapData] = await Promise.all([
        api.settings(),
        api.bootstrap(),
      ]);
      setSecretPlaceholders({
        botToken: settingsData.botToken || "",
        openAIApiKey: settingsData.openAIApiKey || "",
        telegramApiHash: settingsData.telegramApiHash || "",
      });
      setSettings({
        ...settingsData,
        language: normalizeLanguage(settingsData.language),
        openAIOutputMode: settingsData.openAIOutputMode || "auto",
        openAIRequestMode: settingsData.openAIRequestMode || "stream",
        summaryParallelism: settingsData.summaryParallelism || 2,
        summaryRetryLimit: settingsData.summaryRetryLimit ?? 2,
        summaryRetryBackoffBaseMinutes:
          settingsData.summaryRetryBackoffBaseMinutes || 1,
        summaryRetryBackoffMultiplier:
          settingsData.summaryRetryBackoffMultiplier || 3,
        botToken: "",
        openAIApiKey: "",
        telegramApiHash: "",
      });
      setLanguage(normalizeLanguage(settingsData.language));
      setBootstrap(bootstrapData);
      setBotTargetTelegramAccountID((current) => {
        const authorized = telegramAccountsForBotTarget(bootstrapData);
        if (authorized.some((account) => account.id === current)) {
          return current;
        }
        return authorized[0]?.id ?? 0;
      });
      setPendingAuth(bootstrapData.pendingAuth ?? null);
      if (!bootstrapData.pendingAuth && bootstrapData.telegramAuthorized) {
        setAuthEditorOpen(false);
      }
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  async function save(showToast = true) {
    if (!settings) {
      return false;
    }

    try {
      const saved = await api.saveSettings(settings);
      setLanguage(normalizeLanguage(saved.language));
      if (showToast) {
        toast.showSuccess("系统配置已保存。");
      }
      await load();
      notifyBootstrapRefresh();
      return true;
    } catch (err) {
      toast.showError(asMessage(err));
      return false;
    }
  }

  async function testOpenAIConnection() {
    if (!settings || testingOpenAI) {
      return;
    }

    setTestingOpenAI(true);
    try {
      await api.testOpenAISettings(settings);
      toast.showSuccess("OpenAI 连接测试成功。");
    } catch (err) {
      toast.showError(asMessage(err));
    } finally {
      setTestingOpenAI(false);
    }
  }

  async function changeAccessPassword() {
    if (nextAccessPassword.trim().length < 8) {
      toast.showError("访问密码至少需要 8 位。");
      return;
    }
    if (nextAccessPassword !== nextAccessPasswordConfirm) {
      toast.showError("两次输入的访问密码不一致。");
      return;
    }

    try {
      await api.changePassword(currentAccessPassword, nextAccessPassword);
      setCurrentAccessPassword("");
      setNextAccessPassword("");
      setNextAccessPasswordConfirm("");
      toast.showSuccess("访问密码已更新。");
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  async function logout() {
    try {
      await api.logout();
    } catch {
      // ignore logout errors and continue redirecting
    }
    router.replace("/login");
  }

  async function startAuthFlow() {
    if (!settings || authBlocked(authRetryUntil, authRetryNow)) {
      return;
    }
    const saved = await save(false);
    if (!saved) {
      return;
    }

    try {
      const state = await api.startAuth(
        fullPhone(countryCode, phoneNumber),
        reauthAccountId ?? undefined,
      );
      setPendingAuth(state as PendingAuth);
      setCode("");
      setPassword("");
      setAuthEditorOpen(true);
      setAuthRetryUntil(null);
      toast.showSuccess(
        `验证码已发送到 ${fullPhone(countryCode, phoneNumber)}。`,
      );
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function submitCode() {
    if (authBlocked(authRetryUntil, authRetryNow)) {
      return;
    }

    try {
      const response = (await api.verifyCode(code)) as PendingAuth;
      setAuthRetryUntil(null);
      if (response.step === "password") {
        setPendingAuth(response);
        setPassword("");
        toast.showSuccess("该账号开启了两步验证，请继续输入密码。");
        return;
      }
      await finalizeLogin("Telegram 登录成功，群组已同步。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function submitPassword() {
    if (authBlocked(authRetryUntil, authRetryNow)) {
      return;
    }

    try {
      await api.verifyPassword(password);
      setAuthRetryUntil(null);
      await finalizeLogin("两步验证通过，Telegram 登录成功，群组已同步。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function finalizeLogin(prefix: string) {
    setPendingAuth(null);
    setCode("");
    setPassword("");
    await load();
    notifyBootstrapRefresh();
    setAuthEditorOpen(false);
    toast.showSuccess(prefix);
  }

  async function syncChats() {
    try {
      const chats = await api.syncChats();
      await load();
      notifyBootstrapRefresh();
      if (chats.length > 0) {
        toast.showSuccess(`已同步 ${chats.length} 个群组。`);
        return;
      }
      toast.showSuccess("已同步，但当前没有发现可管理的群组。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function syncTelegramAccount(accountId: number) {
    try {
      await api.syncTelegramAccount(accountId);
      await load();
      notifyBootstrapRefresh();
      toast.showSuccess("该账号的群组已同步。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function deleteTelegramAccount(accountId: number) {
	if (!window.confirm("确定删除这个 Telegram 账号吗？已有消息和摘要不会被删除。")) {
	  return;
	}
    try {
      await api.deleteTelegramAccount(accountId);
      await load();
      notifyBootstrapRefresh();
      toast.showSuccess("Telegram 账号已删除。");
    } catch (err) {
      toast.showError(asMessage(err));
    }
  }

  async function resolveBotTargetChat() {
    if (!settings) {
      return;
    }

    const currentSettings = settings;
    if (!bootstrap?.telegramAuthorized) {
      toast.showError("自动获取前请先完成 Telegram 登录。");
      return;
    }
    if (
      !hasAvailableBotToken(
        currentSettings.botToken,
        secretPlaceholders.botToken,
      )
    ) {
      toast.showError("请先填写 Bot Token。");
      return;
    }

    setResolvingBotTargetChat(true);
    try {
      const result = await api.resolveBotTargetChat(
        currentSettings.botToken,
        botTargetTelegramAccountID,
      );
      setBotTargetChatCandidates(result.candidates);
      if (result.candidates.length === 0) {
        toast.showError("未找到最近消息，请先给 Bot 发一条消息后再重试。");
        return;
      }
      if (result.candidates.length === 1) {
        const [candidate] = result.candidates;
        await saveResolvedBotTargetChat(candidate.chatId);
        return;
      }
      toast.showSuccess("找到了多个可能的会话，请选择一个。");
    } catch (err) {
      toast.showError(asMessage(err));
    } finally {
      setResolvingBotTargetChat(false);
    }
  }

  async function selectBotTargetChat(candidate: BotTargetChatCandidate) {
    await saveResolvedBotTargetChat(candidate.chatId);
  }

  async function saveResolvedBotTargetChat(chatId: string) {
    if (!settings) {
      return false;
    }

    setSavingBotTargetChat(true);
    try {
      const persistedSettings = await api.settings();
      const nextSettings = {
        ...persistedSettings,
        botEnabled: settings.botEnabled,
        botTargetChatId: chatId,
        botToken: settings.botToken?.trim() || persistedSettings.botToken || "",
      };
      const saved = await api.saveSettings(nextSettings);
      setSettings((current) => {
        if (!current) {
          return current;
        }
        return {
          ...current,
          botEnabled: saved.botEnabled,
          botTargetChatId: saved.botTargetChatId,
          botToken: "",
        };
      });
      setSecretPlaceholders((current) => ({
        ...current,
        botToken: saved.botToken || current.botToken,
      }));
      setBotTargetChatCandidates([]);
      notifyBootstrapRefresh();
      toast.showSuccess("已自动绑定并保存 Chat ID。");
      return true;
    } catch (err) {
      toast.showError(asMessage(err));
      return false;
    } finally {
      setSavingBotTargetChat(false);
    }
  }

  async function resetAuthEditor() {
    if (pendingAuth?.accountId) {
      try {
        await api.cancelTelegramAuth();
      } catch {
        // The temporary account may already have been removed after an auth error.
      }
    }
    setAuthEditorOpen(false);
    setReauthAccountId(null);
    setPendingAuth(null);
    setCode("");
    setPassword("");
  }

  function toggleAuthEditor(accountId?: number) {
    setReauthAccountId(accountId ?? null);
    setAuthEditorOpen((current) => (accountId ? true : !current));
  }

  function handleAuthError(err: unknown) {
    if (err instanceof APIError && err.retryAfterSeconds) {
      setAuthRetryUntil(Date.now() + err.retryAfterSeconds * 1000);
    }
    toast.showError(asMessage(err));
  }

  function selectTab(tab: SettingsTab) {
    setActiveTab(tab);
    const url = new URL(window.location.href);
    url.searchParams.set("tab", tab);
    window.history.replaceState({}, "", url);
  }

  if (!settings) {
    return (
      <DashboardPage
        title="系统配置"
        description="管理 Telegram App、摘要引擎、偏好设置和 Bot 推送。"
      />
    );
  }

  const stage = resolveAuthStage(bootstrap, pendingAuth, authEditorOpen);
  const blocked = authBlocked(authRetryUntil, authRetryNow);
  const retryLabel = authRetryLabel(authRetryUntil, authRetryNow);

  return (
    <DashboardPage
      title="系统配置"
      description="在这里管理 Telegram App、摘要引擎、偏好设置和 Bot 推送。"
    >
      <SettingsTabs active={activeTab} onChange={selectTab} />

      <div className="settings-tab-content" key={activeTab}>
      <div className="dashboard-workspace settings-workspace settings-workspace-single">
        <div className="settings-column">
          {activeTab === "telegram" ? (
          <Surface
            title="Telegram App"
            description="TGTLDR 会作为第三方 Telegram 客户端登录你的账号。请先创建 Telegram App，再在这里填写 API 凭据。"
          >
            <div className="form-stack">
              <Field
                label="Telegram API ID"
                hint="在 my.telegram.org/apps 创建后获得。"
              >
                <Input
                  value={settings.telegramApiId || ""}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      telegramApiId: Number(event.target.value || "0"),
                    })
                  }
                />
              </Field>
              <Field
                label="Telegram API Hash"
                hint="已保存时会显示掩码。留空表示保持现有值。"
              >
                <Input
                  placeholder={secretPlaceholder(
                    secretPlaceholders.telegramApiHash,
                  )}
                  type="password"
                  value={settings.telegramApiHash || ""}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      telegramApiHash: event.target.value,
                    })
                  }
                />
              </Field>
            </div>
          </Surface>
          ) : null}

          {activeTab === "summary" ? (
          <Surface
            title="摘要引擎"
            description="配置模型、API 地址、输出长度和并行处理方式。"
          >
            <div className="form-stack">
              <Field label="Base URL">
                <Input
                  value={settings.openAIBaseUrl}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      openAIBaseUrl: event.target.value,
                    })
                  }
                />
              </Field>
              <Field label="Model">
                <Input
                  value={settings.openAIModel}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      openAIModel: event.target.value,
                    })
                  }
                />
              </Field>
              <Field
                label="API Key"
                hint="已保存时会显示掩码。留空表示保持现有值。"
              >
                <Input
                  placeholder={secretPlaceholder(
                    secretPlaceholders.openAIApiKey,
                  )}
                  type="password"
                  value={settings.openAIApiKey || ""}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      openAIApiKey: event.target.value,
                    })
                  }
                />
              </Field>
              <Field
                label="Temperature"
                hint="建议范围：0.0-2.0。摘要场景通常建议 0.1-0.7。"
              >
                <Input
                  max="2"
                  min="0"
                  step="0.1"
                  type="number"
                  value={settings.openAITemperature}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      openAITemperature: Number(event.target.value || "0"),
                    })
                  }
                />
              </Field>
              <Field
                label="输出长度"
                hint="自动模式不设置显式输出上限；自定义模式会应用 Max Output Tokens 限制。"
              >
                <AppSelect
                  onChange={(value) =>
                    setSettings({
                      ...settings,
                      openAIOutputMode: value as "auto" | "manual",
                    })
                  }
                  options={[
                    { value: "auto", label: "自动" },
                    { value: "manual", label: "自定义" },
                  ]}
                  value={settings.openAIOutputMode}
                />
              </Field>
              {settings.openAIOutputMode === "manual" ? (
                <Field label="Max Output Tokens">
                  <Input
                    type="number"
                    value={settings.openAIMaxOutputTokens}
                    onChange={(event) =>
                      setSettings({
                        ...settings,
                        openAIMaxOutputTokens: Number(
                          event.target.value || "0",
                        ),
                      })
                    }
                  />
                </Field>
              ) : null}
              <Field
                label="调用方式"
                hint="流式更适合转发站，可降低网关等待完整响应导致的超时风险。"
              >
                <AppSelect
                  onChange={(value) =>
                    setSettings({
                      ...settings,
                      openAIRequestMode: value as "stream" | "non_stream",
                    })
                  }
                  options={[
                    { value: "stream", label: "流式" },
                    { value: "non_stream", label: "非流式" },
                  ]}
                  value={settings.openAIRequestMode || "stream"}
                />
              </Field>
              <Field
                label="并发摘要数"
                hint="最多同时总结多少个消息分块。"
              >
                <AppSelect
                  onChange={(value) =>
                    setSettings({
                      ...settings,
                      summaryParallelism: Number(value),
                    })
                  }
                  options={[
                    { value: "1", label: "1" },
                    { value: "2", label: "2" },
                    { value: "3", label: "3" },
                    { value: "4", label: "4" },
                    { value: "5", label: "5" },
                    { value: "6", label: "6" },
                  ]}
                  value={String(settings.summaryParallelism || 2)}
                />
              </Field>
              <Field
                label="重试次数上限"
                hint="OpenAI 调用失败后最多额外自动重试多少次；0 表示关闭自动重试。"
              >
                <Input
                  min="0"
                  step="1"
                  type="number"
                  value={settings.summaryRetryLimit}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      summaryRetryLimit: Number(event.target.value || "0"),
                    })
                  }
                />
              </Field>
              <Field
                label="重试起始间隔（分钟）"
                hint="第一次自动重试前等待的分钟数。"
              >
                <Input
                  min="1"
                  step="1"
                  type="number"
                  value={settings.summaryRetryBackoffBaseMinutes}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      summaryRetryBackoffBaseMinutes: Number(
                        event.target.value || "0",
                      ),
                    })
                  }
                />
              </Field>
              <Field
                label="退避倍率"
                hint="每次失败后的等待倍数；1 表示固定间隔。"
              >
                <Input
                  min="1"
                  step="0.1"
                  type="number"
                  value={settings.summaryRetryBackoffMultiplier}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      summaryRetryBackoffMultiplier: Number(
                        event.target.value || "0",
                      ),
                    })
                  }
                />
              </Field>
              <div>
                <Button
                  disabled={testingOpenAI}
                  onClick={() =>
                    startTransition(() => void testOpenAIConnection())
                  }
                  type="button"
                  variant="secondary"
                >
                  {testingOpenAI ? "测试中..." : "测试连接"}
                </Button>
              </div>
            </div>
          </Surface>
          ) : null}

          {activeTab === "preferences" ? (
          <Surface
            title="偏好设置"
            description="这些设置会控制摘要日期和定时任务使用的时区。"
          >
            <div className="form-stack">
              <Field label="默认时区">
                <SearchSelect
                  emptyText="没有匹配的时区"
                  onChange={(value) =>
                    setSettings({ ...settings, defaultTimezone: value })
                  }
                  options={timezoneOptions}
                  placeholder="选择默认时区"
                  searchPlaceholder="搜索时区，例如 Asia/Shanghai"
                  value={settings.defaultTimezone}
                />
              </Field>
              <Field label={dict.language.label} hint={dict.language.hint}>
                <AppSelect
                  onChange={(value) => {
                    const language = normalizeLanguage(value);
                    setSettings({ ...settings, language });
                  }}
                  options={[
                    { value: "zh-CN", label: dict.language.zhCN },
                    { value: "en", label: dict.language.en },
                  ]}
                  value={settings.language}
                />
              </Field>
            </div>
          </Surface>
          ) : null}
        </div>

        <div className="settings-column">
          {activeTab === "telegram" ? (
          <Surface
            title="Telegram 账号"
            description="在这里完成登录、重新登录或重新同步群组。"
          >
            <TelegramAccountSection
              blocked={blocked}
              bootstrap={bootstrap}
              code={code}
              countryCode={countryCode}
              onChangeCode={setCode}
              onChangeCountryCode={setCountryCode}
              onChangePassword={setPassword}
              onChangePhoneNumber={setPhoneNumber}
              onRetrySync={syncTelegramAccount}
              onDeleteAccount={deleteTelegramAccount}
              onResetAuthEditor={resetAuthEditor}
              onStartAuth={startAuthFlow}
              onSubmitCode={submitCode}
              onSubmitPassword={submitPassword}
              onToggleAuthEditor={toggleAuthEditor}
              password={password}
              pendingAuth={pendingAuth}
              phoneNumber={phoneNumber}
              reauthAccountId={reauthAccountId}
              retryLabel={retryLabel}
              stage={stage}
            />
          </Surface>
          ) : null}

          {activeTab === "security" ? (
          <Surface
            title="访问密码"
            description="初始化完成后，后台页面和 API 都需要使用这个密码登录。"
          >
            <div className="form-stack">
              <Field label="当前密码">
                <Input
                  autoComplete="current-password"
                  onChange={(event) =>
                    setCurrentAccessPassword(event.target.value)
                  }
                  type="password"
                  value={currentAccessPassword}
                />
              </Field>
              <Field label="新密码" hint="至少 8 位。">
                <Input
                  autoComplete="new-password"
                  onChange={(event) =>
                    setNextAccessPassword(event.target.value)
                  }
                  type="password"
                  value={nextAccessPassword}
                />
              </Field>
              <Field label="确认新密码">
                <Input
                  aria-invalid={
                    nextAccessPasswordConfirm.trim() !== "" &&
                    nextAccessPassword !== nextAccessPasswordConfirm
                  }
                  autoComplete="new-password"
                  onChange={(event) =>
                    setNextAccessPasswordConfirm(event.target.value)
                  }
                  type="password"
                  value={nextAccessPasswordConfirm}
                />
              </Field>
            </div>
            <div className="button-row">
              <Button
                disabled={
                  currentAccessPassword.trim() === "" ||
                  nextAccessPassword.trim().length < 8 ||
                  nextAccessPasswordConfirm.trim() === "" ||
                  nextAccessPassword !== nextAccessPasswordConfirm
                }
                onClick={() => void changeAccessPassword()}
                type="button"
              >
                更新访问密码
              </Button>
              <Button
                onClick={() => void logout()}
                type="button"
                variant="secondary"
              >
                退出登录
              </Button>
            </div>
          </Surface>
          ) : null}

          {activeTab === "bot" ? (
          <Surface
            title="Telegram Bot 推送"
            description="如果你只在网页端看摘要，这一块可以保持关闭。"
          >
            <div className="form-stack">
              <Field label="投递方式">
                <AppSelect
                  onChange={(value) =>
                    setSettings({ ...settings, botEnabled: value === "yes" })
                  }
                  options={[
                    { value: "no", label: "仅网页端查看" },
                    { value: "yes", label: "通过 Telegram Bot 推送" },
                  ]}
                  value={settings.botEnabled ? "yes" : "no"}
                />
              </Field>
              <Field
                label="Bot Token"
                hint="已保存时会显示掩码。留空表示保持现有值。"
              >
                <Input
                  placeholder={secretPlaceholder(secretPlaceholders.botToken)}
                  type="password"
                  value={settings.botToken || ""}
                  onChange={(event) => {
                    setBotTargetChatCandidates([]);
                    setSettings({ ...settings, botToken: event.target.value });
                  }}
                />
              </Field>
              <section aria-labelledby="bot-target-chat-title" className="bot-target-binding">
                <div className="bot-target-binding-heading">
                  <h3 id="bot-target-chat-title">目标 Chat ID</h3>
                </div>
                <Field
                  label="Telegram 账号"
                  hint="用于查找与 Bot 的会话；不影响 Bot 实际发送到的目标。"
                >
                  <AppSelect
                    disabled={telegramAccountsForBotTarget(bootstrap).length <= 1}
                    onChange={(value) => {
                      setBotTargetTelegramAccountID(Number(value));
                      setBotTargetChatCandidates([]);
                    }}
                    options={telegramAccountsForBotTarget(bootstrap).map((account) => ({
                      value: String(account.id),
                      label: describeTelegramAccount(account),
                    }))}
                    value={String(botTargetTelegramAccountID)}
                  />
                </Field>
                <div className="bot-target-chat-field">
                  <p className="muted">
                    先在目标私聊中给 Bot 发一条消息，再点击“获取 Chat ID”自动绑定并保存。
                  </p>
                  {!bootstrap?.telegramAuthorized ? (
                    <p className="field-hint">
                      自动获取前需要先完成上面的 Telegram 登录。
                    </p>
                  ) : null}
                  <div className="button-row">
                    <Button
                      disabled={
                        resolvingBotTargetChat ||
                        savingBotTargetChat ||
                        !bootstrap?.telegramAuthorized ||
                        !hasAvailableBotToken(
                          settings.botToken,
                          secretPlaceholders.botToken,
                        )
                      }
                      onClick={() => void resolveBotTargetChat()}
                      type="button"
                      variant="secondary"
                    >
                      {resolvingBotTargetChat
                        ? "正在获取..."
                        : savingBotTargetChat
                          ? "正在保存..."
                          : "获取 Chat ID"}
                    </Button>
                  </div>
                  {botTargetChatCandidates.length > 1 ? (
                    <div className="bot-chat-candidates">
                      {botTargetChatCandidates.map((candidate) => (
                        <Button
                          className="bot-chat-candidate"
                          key={candidate.chatId}
                          disabled={savingBotTargetChat}
                          onClick={() => void selectBotTargetChat(candidate)}
                          type="button"
                          variant={
                            settings.botTargetChatId === candidate.chatId
                              ? "primary"
                              : "secondary"
                          }
                        >
                          {describeBotChatCandidate(candidate)}
                        </Button>
                      ))}
                    </div>
                  ) : null}
                  <div
                    aria-live="polite"
                    className={`bot-target-chat-value ${settings.botTargetChatId ? "" : "empty"}`}
                  >
                    {settings.botTargetChatId
                      ? `当前已绑定：${settings.botTargetChatId}`
                      : "尚未绑定 Chat ID"}
                  </div>
                </div>
              </section>
            </div>
            <p className="muted">
              如果你只想在网页端查看摘要，可以把 Bot 推送保持关闭。
            </p>
          </Surface>
          ) : null}
        </div>
      </div>
      </div>

      {activeTab !== "security" ? (
      <div className="page-savebar">
        <p className="muted">
          {activeTab === "bot"
            ? "获取 Chat ID 会自动保存；其它 Bot 设置需要点击保存。"
            : "修改后请保存当前设置。"}
        </p>
        <Button onClick={() => startTransition(() => void save(true))}>
          保存当前设置
        </Button>
      </div>
      ) : null}
    </DashboardPage>
  );
}

function SettingsTabs({
  active,
  onChange,
}: {
  active: SettingsTab;
  onChange: (tab: SettingsTab) => void;
}) {
  const activeIndex = settingsTabs.findIndex((tab) => tab.value === active);
  return (
    <div aria-label="系统配置分类" className="settings-tabs" role="tablist">
      <div
        className="settings-tabs-track"
        style={{ "--active-tab-index": activeIndex } as CSSProperties}
      >
        <span aria-hidden="true" className="settings-tab-indicator" />
        {settingsTabs.map((tab) => (
        <button
          aria-selected={active === tab.value}
          className={`settings-tab ${active === tab.value ? "active" : ""}`}
          key={tab.value}
          onClick={() => onChange(tab.value)}
          role="tab"
          type="button"
        >
          {tab.label}
        </button>
        ))}
      </div>
    </div>
  );
}

function readSettingsTab(): SettingsTab {
  if (typeof window === "undefined") {
    return "telegram";
  }
  const value = new URL(window.location.href).searchParams.get("tab");
  return settingsTabs.some((tab) => tab.value === value)
    ? (value as SettingsTab)
    : "telegram";
}

function telegramAccountsForBotTarget(bootstrap: Bootstrap | null) {
  return (bootstrap?.telegramAccounts ?? []).filter(
    (account) => account.status === "authorized",
  );
}

function describeTelegramAccount(account: TelegramAuth) {
  const name = account.telegramName || account.phoneNumber || "Telegram 账号";
  return account.telegramHandle ? `${name} (@${account.telegramHandle})` : name;
}

function TelegramAccountSection({
  blocked,
  bootstrap,
  code,
  countryCode,
  onChangeCode,
  onChangeCountryCode,
  onChangePassword,
  onChangePhoneNumber,
  onRetrySync,
  onDeleteAccount,
  onResetAuthEditor,
  onStartAuth,
  onSubmitCode,
  onSubmitPassword,
  onToggleAuthEditor,
  password,
  pendingAuth,
  phoneNumber,
  reauthAccountId,
  retryLabel,
  stage,
}: {
  blocked: boolean;
  bootstrap: Bootstrap | null;
  code: string;
  countryCode: string;
  onChangeCode: (value: string) => void;
  onChangeCountryCode: (value: string) => void;
  onChangePassword: (value: string) => void;
  onChangePhoneNumber: (value: string) => void;
  onRetrySync: (accountId: number) => void;
  onDeleteAccount: (accountId: number) => void;
  onResetAuthEditor: () => void;
  onStartAuth: () => void;
  onSubmitCode: () => void;
  onSubmitPassword: () => void;
  onToggleAuthEditor: (accountId?: number) => void;
  password: string;
  pendingAuth: PendingAuth | null;
  phoneNumber: string;
  reauthAccountId: number | null;
  retryLabel: string | null;
  stage: AuthStage;
}) {
  const reauthAccount = (bootstrap?.telegramAccounts ?? []).find(
    (account) => account.id === reauthAccountId,
  );
  const editorTitle = reauthAccount
    ? `重新登录 ${describeTelegramAccount(reauthAccount)}`
    : "添加 Telegram 账号";

  return (
    <div className="settings-account-stack">
      {(bootstrap?.telegramAccounts ?? []).map((account) => (
        <div className="telegram-account-row" key={account.id}>
          <div className="telegram-account-identity">
            <span>Telegram 账号</span>
            <strong>{describeTelegramAccount(account)}</strong>
          </div>
          <StatusPill tone={account.status === "authorized" ? "good" : "warn"}>
            {account.status === "authorized" ? "已连接" : "需要重新登录"}
          </StatusPill>
          <div className="telegram-account-actions">
            <Button
              onClick={() => startTransition(() => onRetrySync(account.id))}
              type="button"
              variant="secondary"
            >
              同步群组
            </Button>
            {account.status !== "authorized" ? (
              <Button onClick={() => onToggleAuthEditor(account.id)} type="button">
                重新登录
              </Button>
            ) : null}
            {account.usedByChatCount > 0 ? (
              <span className="account-delete-tooltip">
                <Button disabled type="button" variant="ghost">删除账号</Button>
                <span className="account-delete-tooltip-content" role="tooltip">
                  有 {account.usedByChatCount} 个群组正在使用此账号，请先为这些群组选择其他账号。
                </span>
              </span>
            ) : (
              <Button
                onClick={() => startTransition(() => onDeleteAccount(account.id))}
                type="button"
                variant="ghost"
              >
                删除账号
              </Button>
            )}
          </div>
        </div>
      ))}

      {stage === "summary" ? (
        <div className="button-row">
          <Button onClick={() => onToggleAuthEditor()} type="button">
            添加 Telegram 账号
          </Button>
        </div>
      ) : (
        <section aria-label={editorTitle} className="telegram-auth-editor">
          <div className="telegram-auth-editor-heading">
            <h3>{editorTitle}</h3>
            <Button onClick={onResetAuthEditor} type="button" variant="ghost">
              取消
            </Button>
          </div>
          {stage === "phone" ? (
            <>
              <p className="muted">请输入要登录的 Telegram 手机号。</p>
              <div className="setup-phone-row">
                <Field label="国家码">
                  <Input
                    placeholder="+86"
                    value={countryCode}
                    onChange={(event) => onChangeCountryCode(event.target.value)}
                  />
                </Field>
                <Field label="手机号">
                  <Input
                    placeholder="13800138000"
                    value={phoneNumber}
                    onChange={(event) => onChangePhoneNumber(event.target.value)}
                  />
                </Field>
              </div>
              {retryLabel ? <p className="muted">{retryLabel}</p> : null}
              <div className="button-row">
                <Button disabled={blocked} onClick={() => startTransition(onStartAuth)} type="button">
                  发送验证码
                </Button>
              </div>
            </>
          ) : null}
          {stage === "code" ? (
            <>
              <p className="muted">验证码已发送到 <strong>{pendingAuth?.phoneNumber}</strong>。</p>
              <Field label="验证码">
                <Input
                  placeholder="输入 Telegram 发来的验证码"
                  value={code}
                  onChange={(event) => onChangeCode(event.target.value)}
                />
              </Field>
              {retryLabel ? <p className="muted">{retryLabel}</p> : null}
              <div className="button-row">
                <Button disabled={blocked} onClick={() => startTransition(onSubmitCode)} type="button">
                  继续
                </Button>
              </div>
            </>
          ) : null}
          {stage === "password" ? (
            <>
              <Field label="两步验证密码">
                <Input
                  placeholder="输入你的两步验证密码"
                  type="password"
                  value={password}
                  onChange={(event) => onChangePassword(event.target.value)}
                />
              </Field>
              {retryLabel ? <p className="muted">{retryLabel}</p> : null}
              <div className="button-row">
                <Button disabled={blocked} onClick={() => startTransition(onSubmitPassword)} type="button">
                  完成登录
                </Button>
              </div>
            </>
          ) : null}
        </section>
      )}
    </div>
  );
}

function resolveAuthStage(
  bootstrap: Bootstrap | null,
  pendingAuth: PendingAuth | null,
  authEditorOpen: boolean,
): AuthStage {
  if (pendingAuth?.step === "password") {
    return "password";
  }
  if (pendingAuth?.step === "code") {
    return "code";
  }
  if (authEditorOpen || !bootstrap?.telegramAuthorized) {
    return "phone";
  }
  return "summary";
}

function authBlocked(retryUntil: number | null, now: number) {
  return retryUntil !== null && retryUntil > now;
}

function authRetryLabel(retryUntil: number | null, now: number) {
  if (!authBlocked(retryUntil, now)) {
    return null;
  }
  const retryAt = retryUntil ?? now;
  const seconds = Math.ceil((retryAt - now) / 1000);
  return `Telegram 暂时限制了请求，请在 ${seconds} 秒后重试。`;
}

function fullPhone(countryCode: string, phoneNumber: string) {
  return `${countryCode.trim()}${phoneNumber.trim()}`;
}

function secretPlaceholder(value: string) {
  return value || "留空表示保持现有值";
}

function asMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
