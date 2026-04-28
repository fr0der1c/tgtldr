"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { APIError, api } from "@/lib/api";
import {
  Bootstrap,
  BotTargetChatCandidate,
  Chat,
  PendingAuth,
} from "@/lib/types";
import { hasAvailableBotToken } from "@/lib/bot-target-chat";
import {
  asMessage,
  resolveCurrentStep,
  resolveLoginStage,
  stepEnabled,
} from "@/components/setup-wizard-helpers";
import {
  emptySettings,
  knownOpenAIModels,
  SetupStep,
} from "@/components/setup-wizard-types";
import {
  BotStep,
  ConfigStep,
  LoginStep,
  PasswordStep,
} from "@/components/setup-step-content";
import { SetupStepper } from "@/components/setup-stepper";
import {
  detectBrowserLanguage,
  normalizeLanguage,
  useI18n,
} from "@/lib/i18n";

export function SetupWizard() {
  const router = useRouter();
  const { setLanguage } = useI18n();
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);
  const [settings, setSettings] = useState(() => ({
    ...emptySettings,
    language: detectBrowserLanguage(),
  }));
  const [currentStep, setCurrentStep] = useState<SetupStep>("password");
  const [countryCode, setCountryCode] = useState("+86");
  const [phoneNumber, setPhoneNumber] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [accessPassword, setAccessPassword] = useState("");
  const [accessPasswordConfirm, setAccessPasswordConfirm] = useState("");
  const [pendingAuth, setPendingAuth] = useState<PendingAuth | null>(null);
  const [loginStageOverride, setLoginStageOverride] = useState<
    "phone" | "code" | "password" | "success" | null
  >(null);
  const [discoveredChats, setDiscoveredChats] = useState(0);
  const [authRetryUntil, setAuthRetryUntil] = useState<number | null>(null);
  const [authRetryNow, setAuthRetryNow] = useState(Date.now());
  const [botTokenPlaceholder, setBotTokenPlaceholder] = useState("");
  const [botTargetChatCandidates, setBotTargetChatCandidates] = useState<
    BotTargetChatCandidate[]
  >([]);
  const [resolvingBotTargetChat, setResolvingBotTargetChat] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    void refresh("auto");
  }, []);

  useEffect(() => {
    if (!authRetryUntil || authRetryUntil <= Date.now()) {
      return;
    }

    const timer = window.setInterval(() => {
      setAuthRetryNow(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(timer);
    };
  }, [authRetryUntil]);

  const loginStage = resolveLoginStage(
    bootstrap?.telegramAuthorized ?? false,
    pendingAuth,
    loginStageOverride,
  );
  const authBlocked = authRetryUntil !== null && authRetryUntil > authRetryNow;
  const authBlockedLabel = useMemo(() => {
    if (!authBlocked || authRetryUntil === null) {
      return null;
    }

    const seconds = Math.max(
      1,
      Math.ceil((authRetryUntil - authRetryNow) / 1000),
    );
    return `Telegram 暂时限制了请求，请在 ${seconds} 秒后重试。`;
  }, [authBlocked, authRetryNow, authRetryUntil]);
  const fullPhoneNumber = useMemo(
    () => buildPhoneNumber(countryCode, phoneNumber),
    [countryCode, phoneNumber],
  );

  async function refresh(nextStep: SetupStep | "auto" = "auto") {
    try {
      const data = await api.bootstrap();
      setLanguage(normalizeLanguage(data.language));
      setBootstrap(data);
      if (!data.passwordConfigured) {
        setCurrentStep("password");
        return {
          data,
          fullSettings: { ...emptySettings, language: detectBrowserLanguage() },
          discovered: 0,
        };
      }
      if (!data.authenticated) {
        router.replace("/login");
        return {
          data,
          fullSettings: { ...emptySettings, language: detectBrowserLanguage() },
          discovered: 0,
        };
      }

      const [fullSettings, chats] = await Promise.all([
        api.settings(),
        api.listChats().catch(() => [] as Chat[]),
      ]);
      const discovered = Array.isArray(chats) ? chats.length : 0;
      const language = data.settingsConfigured
        ? normalizeLanguage(fullSettings.language)
        : detectBrowserLanguage();
      setLanguage(language);
      setBotTokenPlaceholder(fullSettings.botToken || "");
      setSettings({
        ...emptySettings,
        ...fullSettings,
        language,
        botToken: "",
      });
      setPendingAuth(data.pendingAuth ?? null);
      setDiscoveredChats(discovered);
      setCurrentStep((prev) =>
        nextStep === "auto" ? resolveCurrentStep(data, prev) : nextStep,
      );
      return { data, fullSettings, discovered };
    } catch (err) {
      setError(asMessage(err));
      return null;
    }
  }

  async function saveSettings(nextStep?: SetupStep, noticeText?: string) {
    setError("");
    setNotice("");
    const configError = validateSettings(settings);
    if (configError) {
      setError(configError);
      return false;
    }

    try {
      const saved = await api.saveSettings(settings);
      setLanguage(normalizeLanguage(saved.language));
      setSettings(saved);
      setNotice(noticeText ?? "配置已保存。");
      await refresh(nextStep ?? "auto");
      return true;
    } catch (err) {
      setError(asMessage(err));
      return false;
    }
  }

  async function startAuthFlow() {
    if (authBlocked) {
      return;
    }

    setError("");
    setNotice("");
    try {
      const state = await api.startAuth(fullPhoneNumber);
      setAuthRetryUntil(null);
      setPendingAuth(state as PendingAuth);
      setLoginStageOverride(null);
      setCode("");
      setPassword("");
      setNotice(`验证码已发送到 ${fullPhoneNumber}。`);
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function submitCode() {
    if (authBlocked) {
      return;
    }

    setError("");
    setNotice("");
    try {
      const response = (await api.verifyCode(code)) as PendingAuth;
      setAuthRetryUntil(null);
      if (response.step === "password") {
        setPendingAuth(response);
        setLoginStageOverride(null);
        setPassword("");
        setNotice("该账号开启了两步验证，请继续输入密码。");
        return;
      }

      await finalizeLogin("Telegram 登录成功。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function submitPassword() {
    if (authBlocked) {
      return;
    }

    setError("");
    setNotice("");
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
    setLoginStageOverride("success");
    setPassword("");
    setCode("");
    const result = await refresh("login");
    if (!result) {
      setNotice(prefix);
      return;
    }

    if (result.discovered > 0) {
      setNotice(`${prefix} 已同步 ${result.discovered} 个群组。`);
      return;
    }

    setNotice(`${prefix} 当前还没有拿到群组列表，你可以稍后重试同步。`);
  }

  async function retryChatSync() {
    if (authBlocked) {
      return;
    }

    setError("");
    setNotice("");
    try {
      const chats = await api.syncChats();
      setAuthRetryUntil(null);
      setLoginStageOverride("success");
      setDiscoveredChats(chats.length);
      await refresh("login");
      if (chats.length > 0) {
        setNotice(`已重新同步 ${chats.length} 个群组。`);
        return;
      }
      setNotice("已重新同步，但当前没有发现可管理的群组。");
    } catch (err) {
      handleAuthError(err);
    }
  }

  async function resolveBotTargetChat() {
    if (!bootstrap?.telegramAuthorized) {
      setError("自动获取前请先完成 Telegram 登录。");
      setNotice("");
      return;
    }
    if (!hasAvailableBotToken(settings.botToken, botTokenPlaceholder)) {
      setError("请先填写 Bot Token。");
      setNotice("");
      return;
    }

    setError("");
    setNotice("");
    setResolvingBotTargetChat(true);
    try {
      const result = await api.resolveBotTargetChat(settings.botToken);
      setBotTargetChatCandidates(result.candidates);
      if (result.candidates.length === 0) {
        setError("未找到最近消息，请先给 Bot 发一条消息后再重试。");
        return;
      }
      if (result.candidates.length === 1) {
        const [candidate] = result.candidates;
        setSettings((current) => ({
          ...current,
          botTargetChatId: candidate.chatId,
        }));
        setBotTargetChatCandidates([]);
        setNotice("已获取目标 Chat ID，点击“保存并完成”后生效。");
        return;
      }
      setNotice("找到了多个可能的会话，请选择一个。");
    } catch (err) {
      setError(asMessage(err));
    } finally {
      setResolvingBotTargetChat(false);
    }
  }

  function selectBotTargetChat(candidate: BotTargetChatCandidate) {
    setError("");
    setNotice("已选择目标 Chat ID，点击“保存并完成”后生效。");
    setSettings((current) => ({
      ...current,
      botTargetChatId: candidate.chatId,
    }));
    setBotTargetChatCandidates([]);
  }

  function changeBotToken(botToken: string) {
    setBotTargetChatCandidates([]);
    setSettings((current) => ({
      ...current,
      botToken,
    }));
  }

  function moveToStep(step: SetupStep) {
    if (!stepEnabled(step, bootstrap)) {
      return;
    }
    setCurrentStep(step);
    setError("");
    setNotice("");
  }

  function forceStep(step: SetupStep) {
    setCurrentStep(step);
    setError("");
    setNotice("");
  }

  function handleAuthError(err: unknown) {
    if (err instanceof APIError && err.retryAfterSeconds) {
      setAuthRetryUntil(Date.now() + err.retryAfterSeconds * 1000);
      setAuthRetryNow(Date.now());
    }
    setError(asMessage(err));
  }

  async function finishSetup() {
    if (
      settings.botEnabled &&
      !hasAvailableBotToken(settings.botToken, botTokenPlaceholder)
    ) {
      setError("启用 Bot 推送时必须填写 Bot Token。");
      setNotice("");
      return;
    }

    const ok = await saveSettings("bot", "配置已保存，正在进入后台。");
    if (ok) {
      router.push("/dashboard/chats");
    }
  }

  async function completePasswordSetup() {
    if (accessPassword.trim().length < 8) {
      setError("访问密码至少需要 8 位。");
      setNotice("");
      return;
    }
    if (accessPassword !== accessPasswordConfirm) {
      setError("两次输入的访问密码不一致。");
      setNotice("");
      return;
    }

    setError("");
    setNotice("");
    try {
      await api.setupPassword(accessPassword);
      setNotice("访问密码已设置，继续完成基础配置。");
      await refresh("config");
    } catch (err) {
      setError(asMessage(err));
    }
  }

  return (
    <main className="page-shell setup-page-shell">
      <div className="setup-stack">
        <SetupStepper
          bootstrap={bootstrap}
          currentStep={currentStep}
          onStepChange={moveToStep}
        />

        {currentStep === "config" ? (
          <ConfigStep
            settings={settings}
            setSettings={setSettings}
            canSave={validateSettings(settings) === null}
            onSaveAndContinue={() =>
              saveSettings("login", "基础配置已保存，进入登录步骤。")
            }
          />
        ) : null}

        {currentStep === "login" ? (
          <LoginStep
            bootstrap={bootstrap}
            loginStage={loginStage}
            countryCode={countryCode}
            setCountryCode={setCountryCode}
            phoneNumber={phoneNumber}
            setPhoneNumber={setPhoneNumber}
            code={code}
            setCode={setCode}
            password={password}
            setPassword={setPassword}
            pendingAuth={pendingAuth}
            discoveredChats={discoveredChats}
            authBlocked={authBlocked}
            authBlockedLabel={authBlockedLabel}
            onBack={() => forceStep("config")}
            onContinueFromPhone={startAuthFlow}
            onSubmitCode={submitCode}
            onSubmitPassword={submitPassword}
            onResetToPhone={() => setLoginStageOverride("phone")}
            onContinueToBot={() => forceStep("bot")}
            onResendCode={startAuthFlow}
            onRetrySyncChats={retryChatSync}
          />
        ) : null}

        {currentStep === "bot" ? (
          <BotStep
            botTargetChatCandidates={botTargetChatCandidates}
            botTokenPlaceholder={botTokenPlaceholder}
            settings={settings}
            resolvingBotTargetChat={resolvingBotTargetChat}
            setSettings={setSettings}
            canContinue={
              !settings.botEnabled ||
              hasAvailableBotToken(settings.botToken, botTokenPlaceholder)
            }
            onBotTokenChange={changeBotToken}
            onBack={() => forceStep("login")}
            onContinue={finishSetup}
            onResolveBotTargetChat={resolveBotTargetChat}
            onSelectBotTargetChat={selectBotTargetChat}
            telegramAuthorized={bootstrap?.telegramAuthorized ?? false}
          />
        ) : null}

        {currentStep === "password" ? (
          <PasswordStep
            accessPassword={accessPassword}
            accessPasswordConfirm={accessPasswordConfirm}
            onChangeAccessPassword={setAccessPassword}
            onChangeAccessPasswordConfirm={setAccessPasswordConfirm}
            onFinish={completePasswordSetup}
          />
        ) : null}

        {notice ? (
          <p aria-live="polite" className="notice good" role="status">
            {notice}
          </p>
        ) : null}
        {error ? (
          <p aria-live="assertive" className="notice bad" role="alert">
            {error}
          </p>
        ) : null}
      </div>
    </main>
  );
}

function buildPhoneNumber(countryCode: string, phoneNumber: string) {
  const normalizedCountryCode = countryCode.trim().replace(/\s+/g, "");
  const normalizedPhoneNumber = phoneNumber.trim().replace(/[\s()-]+/g, "");

  if (normalizedPhoneNumber.startsWith("+")) {
    return normalizedPhoneNumber;
  }
  if (normalizedCountryCode === "") {
    return normalizedPhoneNumber;
  }

  const prefix = normalizedCountryCode.startsWith("+")
    ? normalizedCountryCode
    : `+${normalizedCountryCode}`;

  return `${prefix}${normalizedPhoneNumber}`;
}

function validateSettings(settings: typeof emptySettings) {
  if (settings.telegramApiId === 0) {
    return "请填写 Telegram API ID。";
  }
  if ((settings.telegramApiHash ?? "").trim() === "") {
    return "请填写 Telegram API Hash。";
  }
  if ((settings.openAIApiKey ?? "").trim() === "") {
    return "请填写 OpenAI API Key。";
  }
  const usingKnownModel = knownOpenAIModels.includes(
    settings.openAIModel as (typeof knownOpenAIModels)[number],
  );
  if (!usingKnownModel && settings.openAIModel.trim() === "") {
    return "请填写 Model。";
  }
  if (
    settings.openAIOutputMode !== "auto" &&
    settings.openAIOutputMode !== "manual"
  ) {
    return "请选择有效的输出长度模式。";
  }
  if (
    settings.openAIOutputMode === "manual" &&
    settings.openAIMaxOutputTokens <= 0
  ) {
    return "自定义输出长度时，请填写有效的 Max Output Tokens。";
  }
  if (
    !Number.isFinite(settings.summaryParallelism) ||
    settings.summaryParallelism < 1 ||
    settings.summaryParallelism > 6
  ) {
    return "摘要并行度必须在 1 到 6 之间。";
  }
  return null;
}
