"use client";

import { startTransition } from "react";
import { AppSelect } from "@/components/app-select";
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
import { Button, Card, Field, Input } from "@/components/ui";
import { knownOpenAIModels, LoginStage } from "@/components/setup-wizard-types";

type ConfigStepProps = {
  settings: AppSettings;
  setSettings: (settings: AppSettings) => void;
  canSave: boolean;
  onSaveAndContinue: () => void;
};

export function ConfigStep({
  settings,
  setSettings,
  canSave,
  onSaveAndContinue
}: ConfigStepProps) {
  const customModel = !knownOpenAIModels.includes(
    settings.openAIModel as (typeof knownOpenAIModels)[number]
  );

  return (
    <Card title="基础配置">
      <div className="setup-stage">
        <div className="setup-config-grid">
          <section className="setup-config-panel">
            <div className="setup-config-head">
              <div>
                <h3>Telegram 接入</h3>
                <p className="setup-panel-note">
                  用于登录你的 Telegram 账号并读取已加入群组的消息。
                </p>
              </div>
              <ExternalLink href="https://my.telegram.org/apps">
                申请 API ID / Hash
              </ExternalLink>
            </div>
            <div className="form-grid compact">
              <Field label="Telegram API ID" required>
                <Input
                  aria-invalid={settings.telegramApiId === 0}
                  value={settings.telegramApiId || ""}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      telegramApiId: Number(event.target.value || "0")
                    })
                  }
                />
              </Field>
              <Field label="Telegram API Hash" required>
                <Input
                  aria-invalid={(settings.telegramApiHash ?? "").trim() === ""}
                  value={settings.telegramApiHash || ""}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      telegramApiHash: event.target.value
                    })
                  }
                />
              </Field>
            </div>
          </section>

          <section className="setup-config-panel">
            <div className="setup-config-head">
              <div>
                <h3>OpenAI 摘要</h3>
                <p className="setup-panel-note">用于生成每日摘要内容。</p>
              </div>
              <ExternalLink href="https://platform.openai.com/api-keys">
                创建 API Key
              </ExternalLink>
            </div>
            <div className="form-grid compact">
              <Field label="OpenAI Base URL">
                <Input
                  value={settings.openAIBaseUrl}
                  onChange={(event) =>
                    setSettings({
                      ...settings,
                      openAIBaseUrl: event.target.value
                    })
                  }
                />
              </Field>
              <Field label="OpenAI API Key" required>
                <Input
                  aria-invalid={(settings.openAIApiKey ?? "").trim() === ""}
                  value={settings.openAIApiKey || ""}
                  onChange={(event) =>
                    setSettings({ ...settings, openAIApiKey: event.target.value })
                  }
                />
              </Field>
              <Field label="Model" required>
                <AppSelect
                  onChange={(value) =>
                    setSettings({
                      ...settings,
                      openAIModel: value === "__custom__" ? "" : value
                    })
                  }
                  options={[
                    ...knownOpenAIModels.map((model) => ({
                      value: model,
                      label: model
                    })),
                    { value: "__custom__", label: "自定义模型名" }
                  ]}
                  value={customModel ? "__custom__" : settings.openAIModel}
                />
              </Field>
              {customModel ? (
                <Field
                  label="自定义模型名"
                  hint="如果你使用兼容 OpenAI 的其他服务，可以手动填写模型名。"
                  required
                >
                  <Input
                    aria-invalid={settings.openAIModel.trim() === ""}
                    value={settings.openAIModel}
                    onChange={(event) =>
                      setSettings({ ...settings, openAIModel: event.target.value })
                    }
                  />
                </Field>
              ) : null}
            </div>

            <details className="setup-details">
              <summary>高级参数</summary>
              <div className="form-grid compact">
                <Field label="Temperature">
                  <Input
                    type="number"
                    step="0.1"
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
            </details>
          </section>
        </div>

        <div className="setup-actions">
          <Button
            disabled={!canSave}
            onClick={() => startTransition(onSaveAndContinue)}
            type="button"
          >
            保存并继续
          </Button>
        </div>
      </div>
    </Card>
  );
}

type LoginStepProps = {
  bootstrap: Bootstrap | null;
  loginStage: LoginStage;
  countryCode: string;
  setCountryCode: (value: string) => void;
  phoneNumber: string;
  setPhoneNumber: (value: string) => void;
  code: string;
  setCode: (value: string) => void;
  password: string;
  setPassword: (value: string) => void;
  pendingAuth: PendingAuth | null;
  discoveredChats: number;
  authBlocked: boolean;
  authBlockedLabel: string | null;
  onBack: () => void;
  onContinueFromPhone: () => void;
  onSubmitCode: () => void;
  onSubmitPassword: () => void;
  onResetToPhone: () => void;
  onContinueToBot: () => void;
  onResendCode: () => void;
  onRetrySyncChats: () => void;
};

export function LoginStep(props: LoginStepProps) {
  const {
    bootstrap,
    loginStage,
    countryCode,
    setCountryCode,
    phoneNumber,
    setPhoneNumber,
    code,
    setCode,
    password,
    setPassword,
    pendingAuth,
    discoveredChats,
    authBlocked,
    authBlockedLabel,
    onBack,
    onContinueFromPhone,
    onSubmitCode,
    onSubmitPassword,
    onResetToPhone,
    onContinueToBot,
    onResendCode,
    onRetrySyncChats
  } = props;

  return (
    <Card title="登录 Telegram">
      <div className="setup-stage">
        {loginStage === "phone" ? (
          <>
            <div className="setup-auth-block">
              <p className="setup-auth-title">登录 Telegram</p>
              <p className="setup-helper">
                为了登录并获取消息，我们需要登录你的 Telegram 账号。登录后，
                TGTLDR 会体现为你的一台已登录设备。
              </p>
              <div className="setup-phone-row">
                <Field label="国家码">
                  <Input
                    value={countryCode}
                    onChange={(event) => setCountryCode(event.target.value)}
                    placeholder="+86"
                  />
                </Field>
                <Field label="手机号">
                  <Input
                    value={phoneNumber}
                    onChange={(event) => setPhoneNumber(event.target.value)}
                    placeholder="13800138000"
                  />
                </Field>
              </div>
              {authBlockedLabel ? <p className="setup-helper">{authBlockedLabel}</p> : null}
            </div>
            <div className="setup-actions">
              <Button variant="ghost" onClick={onBack} type="button">
                返回上一步
              </Button>
              <Button
                disabled={authBlocked}
                onClick={() => startTransition(onContinueFromPhone)}
                type="button"
              >
                继续
              </Button>
            </div>
          </>
        ) : null}

        {loginStage === "code" ? (
          <>
            <div className="setup-auth-block">
              <p className="setup-auth-title">输入验证码</p>
              <p className="setup-helper">
                验证码已发送到 <strong>{pendingAuth?.phoneNumber}</strong>。
              </p>
              {authBlockedLabel ? <p className="setup-helper">{authBlockedLabel}</p> : null}
              <Field label="验证码">
                <Input
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  placeholder="输入 Telegram 发来的验证码"
                />
              </Field>
            </div>
            <div className="setup-actions">
              <div className="setup-inline-actions">
                <Button
                  variant="link"
                  disabled={authBlocked}
                  onClick={() => startTransition(onResendCode)}
                  type="button"
                >
                  重新发送验证码
                </Button>
                <Button variant="link" onClick={onResetToPhone} type="button">
                  返回修改手机号
                </Button>
              </div>
              <Button
                disabled={authBlocked}
                onClick={() => startTransition(onSubmitCode)}
                type="button"
              >
                继续
              </Button>
            </div>
          </>
        ) : null}

        {loginStage === "password" ? (
          <>
            <div className="setup-auth-block">
              <p className="setup-auth-title">输入两步验证密码</p>
              {authBlockedLabel ? <p className="setup-helper">{authBlockedLabel}</p> : null}
              <Field label="两步验证密码">
                <Input
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder="输入你的两步验证密码"
                />
              </Field>
            </div>
            <div className="setup-actions">
              <Button variant="link" onClick={onResetToPhone} type="button">
                返回修改手机号
              </Button>
              <Button
                disabled={authBlocked}
                onClick={() => startTransition(onSubmitPassword)}
                type="button"
              >
                继续
              </Button>
            </div>
          </>
        ) : null}

        {loginStage === "success" ? (
          <>
            <div className="setup-auth-block">
              <p className="setup-auth-title">
                {discoveredChats > 0 ? "已登录并同步群组" : "已登录，尚未同步群组"}
              </p>
              {discoveredChats === 0 ? (
                <p className="setup-helper">
                  当前还没有拿到群组列表。若你已经加入了 Telegram 群组，可以重试一次同步。
                </p>
              ) : null}
              <div className="setup-metrics">
                <div className="setup-metric">
                  <span>登录账号</span>
                  <strong>{bootstrap?.auth?.telegramName || "已授权账号"}</strong>
                </div>
                <div className="setup-metric">
                  <span>发现群组</span>
                  <strong>{discoveredChats}</strong>
                </div>
                <div className="setup-metric">
                  <span>已启用消息保存</span>
                  <strong>{bootstrap?.enabledChatCount ?? 0}</strong>
                </div>
              </div>
            </div>
            <div className="setup-actions">
              <Button variant="ghost" onClick={onBack} type="button">
                返回上一步
              </Button>
              <div className="setup-inline-actions">
                {discoveredChats === 0 ? (
                  <Button
                    disabled={authBlocked}
                    onClick={() => startTransition(onRetrySyncChats)}
                    type="button"
                  >
                    重试同步群组
                  </Button>
                ) : null}
                <Button onClick={onContinueToBot} type="button">
                  继续
                </Button>
              </div>
            </div>
          </>
        ) : null}
      </div>
    </Card>
  );
}

type BotStepProps = {
  botTargetChatCandidates: BotTargetChatCandidate[];
  botTokenPlaceholder: string;
  settings: AppSettings;
  resolvingBotTargetChat: boolean;
  setSettings: (settings: AppSettings) => void;
  canFinish: boolean;
  onBotTokenChange: (value: string) => void;
  onBack: () => void;
  onFinish: () => void;
  onResolveBotTargetChat: () => void;
  onSelectBotTargetChat: (candidate: BotTargetChatCandidate) => void;
  telegramAuthorized: boolean;
};

export function BotStep({
  botTargetChatCandidates,
  botTokenPlaceholder,
  settings,
  resolvingBotTargetChat,
  setSettings,
  canFinish,
  onBotTokenChange,
  onBack,
  onFinish,
  onResolveBotTargetChat,
  onSelectBotTargetChat,
  telegramAuthorized
}: BotStepProps) {
  return (
    <Card title="Bot 推送">
      <div className="setup-stage">
        <div className="setup-choice-grid">
          <button
            className={`setup-choice ${settings.botEnabled ? "" : "active"}`}
            onClick={() =>
              setSettings({
                ...settings,
                botEnabled: false
              })
            }
            type="button"
          >
            <span className="setup-choice-title">仅在网页端查看摘要</span>
          </button>
          <button
            className={`setup-choice ${settings.botEnabled ? "active" : ""}`}
            onClick={() =>
              setSettings({
                ...settings,
                botEnabled: true
              })
            }
            type="button"
          >
            <span className="setup-choice-title">同时通过 Telegram Bot 推送</span>
          </button>
        </div>

        {settings.botEnabled ? (
          <div className="setup-bot-fields">
            <Field
              label="Bot Token"
              hint="启用 Bot 推送后必须填写。"
              required
            >
              <Input
                required
                placeholder={botTokenPlaceholder || "启用 Bot 推送后必须填写。"}
                type="password"
                value={settings.botToken || ""}
                onChange={(event) => onBotTokenChange(event.target.value)}
              />
            </Field>
            <Field
              as="div"
              label="目标 Chat ID"
              hint="先给 Bot 发消息，再点击“获取 Chat ID”自动绑定。"
            >
              <div className="bot-target-chat-field">
                <p className="muted">
                  1. 先在目标私聊或群聊里给 Bot 发一条消息。
                  <br />
                  2. 回到这里点击“获取 Chat ID”。
                </p>
                {!telegramAuthorized ? (
                  <p className="field-hint">自动获取前需要先完成上一步的 Telegram 登录。</p>
                ) : null}
                <div className="button-row">
                  <Button
                    disabled={
                      resolvingBotTargetChat ||
                      !telegramAuthorized ||
                      !hasAvailableBotToken(settings.botToken, botTokenPlaceholder)
                    }
                    onClick={() => void onResolveBotTargetChat()}
                    type="button"
                    variant="secondary"
                  >
                    {resolvingBotTargetChat ? "正在获取..." : "获取 Chat ID"}
                  </Button>
                </div>
                {botTargetChatCandidates.length > 1 ? (
                  <div className="bot-chat-candidates">
                    {botTargetChatCandidates.map((candidate) => (
                      <Button
                        className="bot-chat-candidate"
                        key={candidate.chatId}
                        onClick={() => onSelectBotTargetChat(candidate)}
                        type="button"
                        variant={
                          settings.botTargetChatId === candidate.chatId ? "primary" : "secondary"
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
                    ? `当前将绑定：${settings.botTargetChatId}`
                    : "尚未绑定 Chat ID"}
                </div>
                <span className="field-hint">获取成功后会显示在这里，并在“保存并完成”时一起保存。</span>
              </div>
            </Field>
          </div>
        ) : null}

        <div className="setup-actions">
          <Button variant="ghost" onClick={onBack} type="button">
            返回上一步
          </Button>
          <Button
            disabled={!canFinish}
            onClick={() => startTransition(onFinish)}
            type="button"
          >
            保存并完成
          </Button>
        </div>
      </div>
    </Card>
  );
}

function ExternalLink({
  children,
  href
}: {
  children: string;
  href: string;
}) {
  return (
    <a className="setup-inline-link" href={href} rel="noreferrer" target="_blank">
      {children}
    </a>
  );
}
