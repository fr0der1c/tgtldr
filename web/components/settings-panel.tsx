"use client";

import { startTransition, useEffect, useMemo, useState } from "react";
import { APIError, api } from "@/lib/api";
import { AppSelect } from "@/components/app-select";
import { SearchSelect } from "@/components/search-select";
import {
  AppSettings,
  Bootstrap,
  BotTargetChatCandidate,
  PendingAuth
} from "@/lib/types";
import {
  describeBotChatCandidate,
  hasAvailableBotToken
} from "@/lib/bot-target-chat";
import { DashboardPage, Surface } from "@/components/dashboard-page";
import { useToast } from "@/components/toast";
import { Button, Field, Input, StatusPill } from "@/components/ui";
import { listTimezoneOptions } from "@/lib/timezones";

type SecretPlaceholders = {
  botToken: string;
  openAIApiKey: string;
  telegramApiHash: string;
};

type AuthStage = "summary" | "phone" | "code" | "password";

export function SettingsPanel() {
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);
  const [pendingAuth, setPendingAuth] = useState<PendingAuth | null>(null);
  const [secretPlaceholders, setSecretPlaceholders] = useState<SecretPlaceholders>({
    botToken: "",
    openAIApiKey: "",
    telegramApiHash: ""
  });
  const [countryCode, setCountryCode] = useState("+86");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [authEditorOpen, setAuthEditorOpen] = useState(false);
  const [authRetryUntil, setAuthRetryUntil] = useState<number | null>(null);
  const [authRetryNow, setAuthRetryNow] = useState(Date.now());
  const [botTargetChatCandidates, setBotTargetChatCandidates] = useState<
    BotTargetChatCandidate[]
  >([]);
  const [resolvingBotTargetChat, setResolvingBotTargetChat] = useState(false);
  const [savingBotTargetChat, setSavingBotTargetChat] = useState(false);
  const toast = useToast();
  const timezoneOptions = useMemo(() => listTimezoneOptions(), []);

  useEffect(() => {
    void load();
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
        api.bootstrap()
      ]);
      setSecretPlaceholders({
        botToken: settingsData.botToken || "",
        openAIApiKey: settingsData.openAIApiKey || "",
        telegramApiHash: settingsData.telegramApiHash || ""
      });
      setSettings({
        ...settingsData,
        openAIOutputMode: settingsData.openAIOutputMode || "auto",
        summaryParallelism: settingsData.summaryParallelism || 2,
        botToken: "",
        openAIApiKey: "",
        telegramApiHash: ""
      });
      setBootstrap(bootstrapData);
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
      await api.saveSettings(settings);
      if (showToast) {
        toast.showSuccess("系统配置已保存。");
      }
      await load();
      return true;
    } catch (err) {
      toast.showError(asMessage(err));
      return false;
    }
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
      const state = await api.startAuth(fullPhone(countryCode, phoneNumber));
      setPendingAuth(state as PendingAuth);
      setCode("");
      setPassword("");
      setAuthEditorOpen(true);
      setAuthRetryUntil(null);
      toast.showSuccess(`验证码已发送到 ${fullPhone(countryCode, phoneNumber)}。`);
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
      await finalizeLogin("Telegram 登录成功。");
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
      await finalizeLogin("两步验证通过，Telegram 登录成功。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function finalizeLogin(prefix: string) {
    setPendingAuth(null);
    setCode("");
    setPassword("");
    const chats = await api.syncChats();
    await load();
    setAuthEditorOpen(false);
    if (chats.length > 0) {
      toast.showSuccess(`${prefix} 已同步 ${chats.length} 个群组。`);
      return;
    }
    toast.showSuccess(`${prefix} 当前没有发现可管理的群组。`);
  }

  async function syncChats() {
    try {
      const chats = await api.syncChats();
      await load();
      if (chats.length > 0) {
        toast.showSuccess(`已同步 ${chats.length} 个群组。`);
        return;
      }
      toast.showSuccess("已同步，但当前没有发现可管理的群组。");
    } catch (err) {
      handleAuthError(err);
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
    if (!hasAvailableBotToken(currentSettings.botToken, secretPlaceholders.botToken)) {
      toast.showError("请先填写 Bot Token。");
      return;
    }

    setResolvingBotTargetChat(true);
    try {
      const result = await api.resolveBotTargetChat(currentSettings.botToken);
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
        botToken: settings.botToken?.trim() || persistedSettings.botToken || ""
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
          botToken: ""
        };
      });
      setSecretPlaceholders((current) => ({
        ...current,
        botToken: saved.botToken || current.botToken
      }));
      setBotTargetChatCandidates([]);
      toast.showSuccess("已自动绑定并保存 Chat ID。");
      return true;
    } catch (err) {
      toast.showError(asMessage(err));
      return false;
    } finally {
      setSavingBotTargetChat(false);
    }
  }

  function resetAuthEditor() {
    setAuthEditorOpen(false);
    setPendingAuth(null);
    setCode("");
    setPassword("");
  }

  function handleAuthError(err: unknown) {
    if (err instanceof APIError && err.retryAfterSeconds) {
      setAuthRetryUntil(Date.now() + err.retryAfterSeconds * 1000);
    }
    toast.showError(asMessage(err));
  }

  if (!settings) {
    return (
      <DashboardPage
        title="系统配置"
        description="管理 Telegram 接入、摘要引擎和 Bot 推送。"
      />
    );
  }

  const stage = resolveAuthStage(bootstrap, pendingAuth, authEditorOpen);
  const blocked = authBlocked(authRetryUntil, authRetryNow);
  const retryLabel = authRetryLabel(authRetryUntil, authRetryNow);

  return (
    <DashboardPage
      title="系统配置"
      description="在这里管理 Telegram 接入、摘要模型、默认时区和 Bot 推送。"
    >
      <div className="dashboard-workspace settings-workspace">
        <div className="settings-column">
          <Surface title="Telegram 接入" description="用于登录 Telegram 账号并读取群组消息。">
            <div className="form-grid">
              <Field label="Telegram API ID" hint="在 my.telegram.org/apps 申请后获得。">
                <Input
                  value={settings.telegramApiId || ""}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      telegramApiId: Number(event.target.value || "0")
                    })
                  }
                />
              </Field>
              <Field
                label="Telegram API Hash"
                hint="已保存时会显示掩码。留空表示保持现有值。"
              >
                <Input
                  placeholder={secretPlaceholder(secretPlaceholders.telegramApiHash)}
                  type="password"
                  value={settings.telegramApiHash || ""}
                  onChange={(event) =>
                    setSettings({ ...settings, telegramApiHash: event.target.value })
                  }
                />
              </Field>
            </div>
          </Surface>

          <Surface title="摘要引擎" description="这些参数决定摘要模型、接口地址、输出策略和并行处理方式。">
            <div className="form-grid">
              <Field label="Base URL">
                <Input
                  value={settings.openAIBaseUrl}
                  onChange={(event) =>
                    setSettings({ ...settings, openAIBaseUrl: event.target.value })
                  }
                />
              </Field>
              <Field label="Model">
                <Input
                  value={settings.openAIModel}
                  onChange={(event) =>
                    setSettings({ ...settings, openAIModel: event.target.value })
                  }
                />
              </Field>
              <Field
                label="API Key"
                hint="已保存时会显示掩码。留空表示保持现有值。"
              >
                <Input
                  placeholder={secretPlaceholder(secretPlaceholders.openAIApiKey)}
                  type="password"
                  value={settings.openAIApiKey || ""}
                  onChange={(event) =>
                    setSettings({ ...settings, openAIApiKey: event.target.value })
                  }
                />
              </Field>
              <Field
                label="Temperature"
                hint="建议范围 0.0 - 2.0；摘要场景通常建议 0.1 - 0.7。"
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
                      openAITemperature: Number(event.target.value || "0")
                    })
                  }
                />
              </Field>
              <Field
                label="输出长度"
                hint="自动模式会让系统不显式限制输出长度，自定义模式才会传 Max Output Tokens。"
              >
                <AppSelect
                  onChange={(value) =>
                    setSettings({
                      ...settings,
                      openAIOutputMode: value as "auto" | "manual"
                    })
                  }
                  options={[
                    { value: "auto", label: "自动" },
                    { value: "manual", label: "自定义" }
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
                        openAIMaxOutputTokens: Number(event.target.value || "0")
                      })
                    }
                  />
                </Field>
              ) : null}
              <Field
                label="摘要并行度"
                hint="当一天消息被拆成多个分块时，最多同时生成多少个阶段摘要。"
              >
                <AppSelect
                  onChange={(value) =>
                    setSettings({
                      ...settings,
                      summaryParallelism: Number(value)
                    })
                  }
                  options={[
                    { value: "1", label: "1" },
                    { value: "2", label: "2" },
                    { value: "3", label: "3" },
                    { value: "4", label: "4" },
                    { value: "5", label: "5" },
                    { value: "6", label: "6" }
                  ]}
                  value={String(settings.summaryParallelism || 2)}
                />
              </Field>
            </div>
          </Surface>

          <Surface title="系统行为" description="这些设置会影响摘要日期计算和系统调度行为。">
            <div className="form-grid">
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
            </div>
          </Surface>
        </div>

        <div className="settings-column">
          <Surface title="Telegram 账号" description="在这里完成登录、重新登录或重新同步群组。">
            <TelegramAccountSection
              blocked={blocked}
              bootstrap={bootstrap}
              code={code}
              countryCode={countryCode}
              onChangeCode={setCode}
              onChangeCountryCode={setCountryCode}
              onChangePassword={setPassword}
              onChangePhoneNumber={setPhoneNumber}
              onRetrySync={syncChats}
              onResetAuthEditor={resetAuthEditor}
              onStartAuth={startAuthFlow}
              onSubmitCode={submitCode}
              onSubmitPassword={submitPassword}
              onToggleAuthEditor={() => setAuthEditorOpen((current) => !current)}
              password={password}
              pendingAuth={pendingAuth}
              phoneNumber={phoneNumber}
              retryLabel={retryLabel}
              stage={stage}
            />
          </Surface>

          <Surface title="Bot 推送" description="如果你只在网页端看摘要，这一块可以保持关闭。">
            <div className="form-grid">
              <Field label="是否启用">
                <AppSelect
                  onChange={(value) =>
                    setSettings({ ...settings, botEnabled: value === "yes" })
                  }
                  options={[
                    { value: "no", label: "关闭" },
                    { value: "yes", label: "开启" }
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
              <Field
                as="div"
                label="目标 Chat ID"
                hint="先给 Bot 发消息，再点击“获取 Chat ID”自动绑定并保存。"
              >
                <div className="bot-target-chat-field">
                  <p className="muted">
                    1. 先在目标私聊或群聊里给 Bot 发一条消息。
                    <br />
                    2. 回到这里点击“获取 Chat ID”。
                  </p>
                  {!bootstrap?.telegramAuthorized ? (
                    <p className="field-hint">自动获取前需要先完成上面的 Telegram 登录。</p>
                  ) : null}
                  <div className="button-row">
                    <Button
                      disabled={
                        resolvingBotTargetChat ||
                        savingBotTargetChat ||
                        !bootstrap?.telegramAuthorized ||
                        !hasAvailableBotToken(
                          settings.botToken,
                          secretPlaceholders.botToken
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
                  <span className="field-hint">获取成功后会自动保存并立即显示在这里。</span>
                </div>
              </Field>
            </div>
            <p className="muted">如果你只想在网页端查看摘要，可以把 Bot 推送保持关闭。</p>
          </Surface>
        </div>
      </div>

      <div className="page-savebar">
        <p className="muted">
          获取 Chat ID 会自动保存；其它系统配置修改仍需在这里统一保存。
        </p>
        <Button onClick={() => startTransition(() => void save(true))}>保存系统配置</Button>
      </div>
    </DashboardPage>
  );
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
  onResetAuthEditor,
  onStartAuth,
  onSubmitCode,
  onSubmitPassword,
  onToggleAuthEditor,
  password,
  pendingAuth,
  phoneNumber,
  retryLabel,
  stage
}: {
  blocked: boolean;
  bootstrap: Bootstrap | null;
  code: string;
  countryCode: string;
  onChangeCode: (value: string) => void;
  onChangeCountryCode: (value: string) => void;
  onChangePassword: (value: string) => void;
  onChangePhoneNumber: (value: string) => void;
  onRetrySync: () => void;
  onResetAuthEditor: () => void;
  onStartAuth: () => void;
  onSubmitCode: () => void;
  onSubmitPassword: () => void;
  onToggleAuthEditor: () => void;
  password: string;
  pendingAuth: PendingAuth | null;
  phoneNumber: string;
  retryLabel: string | null;
  stage: AuthStage;
}) {
  if (stage === "summary") {
    return (
      <div className="settings-account-stack">
        <div className="settings-overview-grid">
          <div className="settings-overview-item">
            <span>当前账号</span>
            <strong>{bootstrap?.auth?.telegramName ?? "未连接"}</strong>
          </div>
          <div className="settings-overview-item">
            <span>连接状态</span>
            <StatusPill tone={bootstrap?.telegramAuthorized ? "good" : "warn"}>
              {bootstrap?.telegramAuthorized ? "已连接" : "未连接"}
            </StatusPill>
          </div>
        </div>
        <div className="button-row">
          <Button onClick={() => startTransition(onRetrySync)} type="button" variant="secondary">
            重新同步群组
          </Button>
          <Button onClick={onToggleAuthEditor} type="button">
            重新登录 Telegram
          </Button>
        </div>
      </div>
    );
  }

  if (stage === "phone") {
    return (
      <div className="settings-account-stack">
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
          <Button onClick={onToggleAuthEditor} type="button" variant="secondary">
            收起
          </Button>
          <Button disabled={blocked} onClick={() => startTransition(onStartAuth)} type="button">
            发送验证码
          </Button>
        </div>
      </div>
    );
  }

  if (stage === "code") {
    return (
      <div className="settings-account-stack">
        <p className="muted">
          验证码已发送到 <strong>{pendingAuth?.phoneNumber}</strong>。
        </p>
        <Field label="验证码">
          <Input
            placeholder="输入 Telegram 发来的验证码"
            value={code}
            onChange={(event) => onChangeCode(event.target.value)}
          />
        </Field>
        {retryLabel ? <p className="muted">{retryLabel}</p> : null}
        <div className="button-row">
          <Button onClick={onResetAuthEditor} type="button" variant="secondary">
            取消
          </Button>
          <Button disabled={blocked} onClick={() => startTransition(onSubmitCode)} type="button">
            继续
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="settings-account-stack">
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
          <Button onClick={onResetAuthEditor} type="button" variant="secondary">
            取消
          </Button>
        <Button disabled={blocked} onClick={() => startTransition(onSubmitPassword)} type="button">
          完成登录
        </Button>
      </div>
    </div>
  );
}

function resolveAuthStage(
  bootstrap: Bootstrap | null,
  pendingAuth: PendingAuth | null,
  authEditorOpen: boolean
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
