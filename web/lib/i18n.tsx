"use client";

import {
  createContext,
  PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

export type Language = "zh-CN" | "en";

type DictShape<T> = {
  [K in keyof T]: T[K] extends string ? string : DictShape<T[K]>;
};

export const zh = {
  language: {
    label: "界面语言",
    hint: "控制网页端、默认摘要输出和 Bot 推送文案。",
    zhCN: "中文",
    en: "English",
  },
} as const;

export const en = {
  language: {
    label: "Language",
    hint: "Sets the language for the web UI, default summaries, and Bot delivery messages.",
    zhCN: "中文",
    en: "English",
  },
} satisfies DictShape<typeof zh>;

type I18nContextValue = {
  language: Language;
  setLanguage: (language: Language) => void;
  dict: DictShape<typeof zh>;
};

const I18nContext = createContext<I18nContextValue | null>(null);

const textTranslations = {
  "正在检查当前状态...": "Checking current status...",
  "正在进入首次配置...": "Opening first-time setup...",
  "正在进入登录...": "Opening login...",
  "正在进入后台...": "Opening dashboard...",
  "无法读取当前状态，正在进入首次配置...": "Could not read current status. Opening first-time setup...",
  "关闭": "Close",
  "关闭提示": "Dismiss notification",
  "必填": "Required",
  "没有匹配项": "No matches",
  "Telegram 群组监听与每日摘要平台": "Telegram group monitoring and daily summary platform",
  "访问登录": "Access Login",
  "请输入你在首次设置中配置的访问密码。": "Enter the access password configured during first-time setup.",
  "访问密码": "Access password",
  "登录": "Log in",
  "登录失败。": "Login failed.",
  "无法读取当前状态。": "Could not read current status.",
  "Too long, don't read. 为你每天节省时间。": "Too long, don't read. Save time every day.",
  "群组": "Chats",
  "摘要": "Summaries",
  "系统配置": "Settings",
  "已连接": "Connected",
  "未连接": "Not connected",
  "Bot 推送": "Telegram Bot Delivery",
  "启用中": "Enabled",
  "未启用": "Disabled",
  "在这里选择需要保存消息和生成摘要的群组，并调整每个群的摘要设置。": "Choose which chats should save messages and generate summaries, then tune each chat's summary settings.",
  "已同步群组": "Synced Chats",
  "最新": "Latest",
  "当前 Telegram 账号下可管理的群组与超级群组。": "Groups and supergroups available to the current Telegram account.",
  "已启用消息保存": "Message Saving Enabled",
  "运行中": "Running",
  "启用后，系统会实时保存该群的消息。": "When enabled, the system saves messages from this chat in real time.",
  "已启用 AI 总结": "AI Summary Enabled",
  "已配置": "Configured",
  "只有启用 AI 总结的群组才会参与每日摘要。": "Only chats with AI summaries enabled are included in daily summaries.",
  "群组列表": "Chat List",
  "先查看每个群当前是否启用，再通过操作按钮调整摘要规则、补充群聊背景或回补历史消息。": "Review each chat's status, then use actions to adjust summary rules, add context, or backfill history.",
  "搜索群组": "Search chats",
  "按群名搜索": "Search by chat name",
  "群类型": "Chat type",
  "全部": "All",
  "超级群组": "Supergroup",
  "消息保存": "Message saving",
  "已启用": "Enabled",
  "AI 总结": "AI summary",
  "没有匹配的群组": "No matching chats",
  "调整筛选条件后再试一次。": "Adjust the filters and try again.",
  "群组名称": "Chat name",
  "操作": "Actions",
  "无公开用户名": "No public username",
  "收起编辑": "Collapse editor",
  "编辑群组配置": "Edit chat settings",
  "收起历史消息回补": "Collapse history backfill",
  "加载历史消息": "Load history",
  "启用": "Enable",
  "停用": "Disable",
  "AI 总结交付方式": "AI summary delivery",
  "仅在网页端查看": "View in web app only",
  "通过 Bot 推送": "Deliver through Bot",
  "摘要时间": "Summary time",
  "模型 override": "Model override",
  "例如 gpt-4.1-mini": "e.g. gpt-4.1-mini",
  "验证机器人\n@verify_bot": "verification-bot\n@verify_bot",
  "验证机器人 @verify_bot": "verification-bot @verify_bot",
  "请完成入群验证\n验证已过期": "Please complete group verification\nVerification expired",
  "请完成入群验证 验证已过期": "Please complete group verification Verification expired",
  "例如：这个群主要讨论二级市场和链上项目；ATL 指 All Time Low；喊单通常是半开玩笑表达。": "Example: This group mainly discusses secondary markets and on-chain projects; ATL means All Time Low; \"calls\" are often half-joking trading ideas.",
  "告诉模型你希望如何总结这个群的消息，例如保留决策、行动项和重要链接。": "Tell the model how you want this chat summarized, such as preserving decisions, action items, and important links.",
  "保留机器人消息": "Keep bot messages",
  "保留": "Keep",
  "过滤": "Filter",
  "模型 override 留空时会跟随系统默认模型。": "When model override is empty, the system default model is used.",
  "过滤发言人": "Filtered senders",
  "每行一个，支持昵称或 @username，精确匹配。": "One per line. Supports display names or @username, matched exactly.",
  "过滤关键词": "Filtered keywords",
  "每行一个，按包含关系过滤消息内容。": "One per line. Messages are filtered by substring match.",
  "群聊背景": "Chat context",
  "补充群里常见黑话、术语、长期背景或默认语境，帮助模型正确理解讨论内容。": "Add jargon, terms, long-running context, or default assumptions so the model can understand the discussion.",
  "摘要额外要求": "Extra summary requirements",
  "保存该群配置": "Save chat settings",
  "回补后，这个群较早时间段的消息也可以用于生成摘要。": "After backfill, older messages from this chat can also be used for summaries.",
  "时间范围": "Time range",
  "最近 7 天": "Last 7 days",
  "最近 30 天": "Last 30 days",
  "自定义日期范围": "Custom date range",
  "开始日期": "Start date",
  "结束日期": "End date",
  "摘要记录": "Summary Records",
  "在这里搜索历史摘要、筛选状态，并在需要时手动补跑。": "Search historical summaries, filter by status, and run manual backfills when needed.",
  "累计": "Total",
  "已经写入数据库的摘要任务与结果。": "Summary tasks and results written to the database.",
  "生成成功": "Succeeded",
  "状态为 succeeded 的摘要数量。": "Number of summaries with succeeded status.",
  "正常": "Normal",
  "暂无": "None",
  "处理中": "Processing",
  "当前正在运行或等待完成的摘要。": "Summaries currently running or waiting to complete.",
  "空闲": "Idle",
  "生成失败": "Failed",
  "失败任务建议重新执行，并检查模型配置或上下文限制。": "Retry failed tasks and check model configuration or context limits.",
  "需排查": "Needs attention",
  "稳定": "Stable",
  "搜索摘要关键词": "Search summary keywords",
  "全部群组": "All chats",
  "生成状态": "Generation status",
  "全部状态": "All statuses",
  "成功": "Succeeded",
  "失败": "Failed",
  "等待中": "Pending",
  "发送状态": "Delivery status",
  "已发送": "Sent",
  "待发送": "Pending",
  "发送失败": "Delivery failed",
  "不发送": "Do not send",
  "手动补跑": "Manual run",
  "收起补跑": "Collapse manual run",
  "只会显示已启用 AI 总结的群组。": "Only chats with AI summaries enabled are shown.",
  "还没有可补跑的群组": "No chats available for manual run",
  "只有已启用 AI 总结的群组才会出现在这里。": "Only chats with AI summaries enabled appear here.",
  "选择群组": "Select chat",
  "日期": "Date",
  "立即生成": "Generate now",
  "没有匹配的摘要": "No matching summaries",
  "还没有摘要记录": "No summaries yet",
  "换个关键词或调整筛选条件后再试一次。": "Try another keyword or adjust filters.",
  "先展开手动补跑触发一次摘要，或者等待定时任务执行。": "Use manual run to generate a summary, or wait for the scheduled task.",
  "未知群组": "Unknown chat",
  "未记录模型": "Model not recorded",
  "上一页": "Previous",
  "下一页": "Next",
  "未发送": "Not sent",
  "当前群组设置为不通过 Bot 推送。": "This chat is configured not to deliver through Bot.",
  "Bot 配置尚未完成，当前无法发送。": "Bot configuration is incomplete, so this cannot be sent now.",
  "摘要尚未生成成功，当前不会执行发送。": "The summary has not succeeded yet, so delivery will not run.",
  "摘要已生成，等待自动发送或手动重试。": "The summary is ready and waiting for automatic delivery or manual retry.",
  "没有可查看的摘要": "No summary selected",
  "从列表中选择一条摘要后，这里会展示完整正文。": "Select a summary from the list to view the full content here.",
  "通过 Bot 发送": "Send through Bot",
  "查看原始 prompt": "View raw prompt",
  "重新生成": "Regenerate",
  "还没有摘要内容": "No summary content yet",
  "这条摘要还没有正文，请稍后重试或重新生成。": "This summary has no content yet. Try again later or regenerate it.",
  "原始 prompt": "Raw prompt",
  "查看这条摘要按当前规则重建出的完整 AI 输入上下文。": "View the full AI input context rebuilt for this summary using current rules.",
  "正在加载上下文预览…": "Loading context preview...",
  "暂时无法生成上下文预览": "Could not generate context preview",
  "稍后重试，或先确认这条摘要对应的消息仍然存在。": "Try again later, or confirm that the messages for this summary still exist.",
  "系统提示词": "System prompt",
  "合并提示词": "Merge prompt",
  "最终汇总阶段": "Final merge stage",
  "欢迎使用，请先完成设置向导": "Welcome. Complete the setup wizard first",
  "第 1 步": "Step 1",
  "第 2 步": "Step 2",
  "第 3 步": "Step 3",
  "第 4 步": "Step 4",
  "基础配置": "Basic Configuration",
  "Telegram 接入": "Telegram App",
  "Telegram App": "Telegram App",
  "用于登录你的 Telegram 账号并读取已加入群组的消息。": "TGTLDR signs in as a third-party Telegram client. Create a Telegram App first, then enter its API credentials here.",
  "TGTLDR 会作为第三方 Telegram 客户端登录你的账号。请先创建 Telegram App，再在这里填写 API 凭据。": "TGTLDR signs in as a third-party Telegram client. Create a Telegram App first, then enter its API credentials here.",
  "申请 API ID / Hash": "Apply for API ID / Hash",
  "OpenAI 摘要": "OpenAI Summary",
  "用于生成每日摘要内容。": "Used to generate daily summary content.",
  "创建 API Key": "Create API Key",
  "自定义模型名": "Custom model name",
  "如果你使用兼容 OpenAI 的其他服务，可以手动填写模型名。": "If you use another OpenAI-compatible service, enter the model name manually.",
  "高级参数": "Advanced parameters",
  "输出长度": "Output length",
  "自动模式会让系统不显式限制输出长度，自定义模式才会传 Max Output Tokens。": "Auto mode does not set an explicit output limit. Custom mode applies the Max Output Tokens limit.",
  "自动模式不设置显式输出上限；自定义模式会应用 Max Output Tokens 限制。": "Auto mode does not set an explicit output limit. Custom mode applies the Max Output Tokens limit.",
  "自动": "Auto",
  "自定义": "Custom",
  "摘要并行度": "Concurrent summaries",
  "并发摘要数": "Concurrent summaries",
  "当一天消息被拆成多个分块时，最多同时生成多少个阶段摘要。": "Maximum number of chunks to summarize at the same time.",
  "最多同时总结多少个消息分块。": "Maximum number of chunks to summarize at the same time.",
  "保存并继续": "Save and continue",
  "登录 Telegram": "Log in to Telegram",
  "为了登录并获取消息，我们需要登录你的 Telegram 账号。登录后，": "To read messages, TGTLDR needs to log in to your Telegram account. After login,",
  "TGTLDR 会体现为你的一台已登录设备。": "TGTLDR appears as one of your logged-in devices.",
  "国家码": "Country code",
  "手机号": "Phone number",
  "返回上一步": "Back",
  "继续": "Continue",
  "输入验证码": "Enter verification code",
  "验证码": "Verification code",
  "输入 Telegram 发来的验证码": "Enter the code from Telegram",
  "重新发送验证码": "Resend code",
  "返回修改手机号": "Change phone number",
  "输入两步验证密码": "Enter two-step verification password",
  "两步验证密码": "Two-step verification password",
  "输入你的两步验证密码": "Enter your two-step verification password",
  "已登录并同步群组": "Logged in and chats synced",
  "已登录，尚未同步群组": "Logged in, chats not synced yet",
  "当前还没有拿到群组列表。若你已经加入了 Telegram": "No chat list is available yet. If you have joined Telegram",
  "群组，可以重试一次同步。": "groups, you can try syncing again.",
  "登录账号": "Account",
  "已授权账号": "Authorized account",
  "发现群组": "Discovered chats",
  "重试同步群组": "Retry chat sync",
  "同时通过 Telegram Bot 推送": "Also deliver through Telegram Bot",
  "启用 Bot 推送后必须填写。": "Required when Bot delivery is enabled.",
  "目标 Chat ID": "Target Chat ID",
  "先给 Bot 发消息，再点击“获取 Chat ID”自动绑定。": "Send the Bot a message first, then click \"Get Chat ID\" to bind automatically.",
  "获取 Chat ID": "Get Chat ID",
  "正在获取...": "Fetching...",
  "尚未绑定 Chat ID": "Chat ID not bound yet",
  "获取成功后会显示在这里，并在“保存并完成”时一起保存。": "After successful lookup, it appears here and is saved when setup is completed.",
  "保存并完成": "Save and finish",
  "设置访问密码": "Set access password",
  "访问保护": "Access protection",
  "确认访问密码": "Confirm access password",
  "两次输入的密码不一致。": "The two passwords do not match.",
  "设置密码并继续": "Set password and continue",
  "默认时区": "Default timezone",
  "选择默认时区": "Select default timezone",
  "搜索时区，例如 Asia/Shanghai": "Search timezone, e.g. Asia/Shanghai",
  "没有匹配的时区": "No matching timezones",
  "管理 Telegram 接入、摘要引擎和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "管理 Telegram App、摘要引擎、偏好设置和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "在这里管理 Telegram 接入、摘要模型、默认时区和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "在这里管理 Telegram App、摘要引擎、偏好设置和 Bot 推送。": "Manage your Telegram app credentials, summary engine, preferences, and Bot delivery.",
  "摘要引擎": "Summary Engine",
  "这些参数决定摘要模型、接口地址、输出策略和并行处理方式。": "Configure the model, API endpoint, output length, and parallel processing.",
  "配置模型、API 地址、输出长度和并行处理方式。": "Configure the model, API endpoint, output length, and parallel processing.",
  "系统行为": "Preferences",
  "偏好设置": "Preferences",
  "这些设置会影响摘要日期计算和系统调度行为。": "These settings control the timezone used for summary dates and schedules.",
  "这些设置会影响摘要日期计算、网页文案和默认输出语言。": "These settings control the timezone used for summary dates, schedules, and the default output language.",
  "这些设置会控制摘要日期和定时任务使用的时区。": "These settings control the timezone used for summary dates and schedules.",
  "这些设置会控制摘要日期、定时任务和默认输出语言。": "These settings control the timezone used for summary dates, schedules, and the default output language.",
  "Telegram 账号": "Telegram Account",
  "在这里完成登录、重新登录或重新同步群组。": "Log in, log in again, or resync chats here.",
  "初始化完成后，后台页面和 API 都需要使用这个密码登录。": "After initialization, dashboard pages and APIs require this password.",
  "用于登录 Telegram 账号并读取群组消息。": "TGTLDR signs in as a third-party Telegram client. Create a Telegram App first, then enter its API credentials here.",
  "在 my.telegram.org/apps 申请后获得。": "Created at my.telegram.org/apps.",
  "在 my.telegram.org/apps 创建后获得。": "Created at my.telegram.org/apps.",
  "已保存时会显示掩码。留空表示保持现有值。": "Saved secrets are masked. Leave blank to keep the current value.",
  "建议范围 0.0 - 2.0；摘要场景通常建议 0.1 - 0.7。": "Suggested range: 0.0-2.0. Summaries usually work best around 0.1-0.7.",
  "建议范围：0.0-2.0。摘要场景通常建议 0.1-0.7。": "Suggested range: 0.0-2.0. Summaries usually work best around 0.1-0.7.",
  "当前密码": "Current password",
  "新密码": "New password",
  "至少 8 位。": "At least 8 characters.",
  "至少 8 位。后续可以在系统配置中修改。": "At least 8 characters. You can change it later in Settings.",
  "确认新密码": "Confirm new password",
  "更新访问密码": "Update access password",
  "退出登录": "Log out",
  "Telegram Bot 推送": "Telegram Bot Delivery",
  "是否启用": "Delivery mode",
  "投递方式": "Delivery mode",
  "关闭 Bot 推送": "Web app only",
  "开启 Bot 推送": "Send via Telegram Bot",
  "仅网页端查看": "Web app only",
  "通过 Telegram Bot 推送": "Send via Telegram Bot",
  "如果你只在网页端看摘要，这一块可以保持关闭。": "Keep this off if you only read summaries in the web app.",
  "先给 Bot 发消息，再点击“获取 Chat ID”自动绑定并保存。": "Send a message to the Bot first, then click \"Get Chat ID\" to bind the target chat automatically.",
  "1. 先在目标私聊或群聊里给 Bot 发一条消息。": "1. Send a message to the Bot in the target private chat or group.",
  "2. 回到这里点击“获取 Chat ID”。": "2. Return here and click \"Get Chat ID\".",
  "自动获取前需要先完成上一步的 Telegram 登录。": "Complete the previous Telegram login step before automatic lookup.",
  "自动获取前需要先完成上面的 Telegram 登录。": "Complete the Telegram login above before automatic lookup.",
  "正在保存...": "Saving...",
  "获取成功后会自动保存并立即显示在这里。": "After successful lookup, it is saved automatically and shown here.",
  "如果你只想在网页端查看摘要，可以把 Bot 推送保持关闭。": "Keep Bot delivery off if you only read summaries in the web app.",
  "获取 Chat ID 会自动保存；其它系统配置修改仍需在这里统一保存。": "Chat ID binding is saved automatically. Save other settings with the button below.",
  "保存系统配置": "Save settings",
  "当前账号": "Current account",
  "已自动绑定并保存 Chat ID。": "Chat ID bound and saved automatically.",
  "连接状态": "Connection status",
  "重新同步群组": "Resync chats",
  "重新登录 Telegram": "Reconnect Telegram",
  "发送验证码": "Send code",
  "取消": "Cancel",
  "完成登录": "Complete login",
  "留空表示保持现有值": "Leave empty to keep the current value",
  "私聊": "Private chat",
  "群聊": "Group",
  "频道": "Channel",
  "会话": "Chat",
  "超级群": "Supergroup",
  "协调世界时": "Coordinated Universal Time",
  "界面语言": "Language",
  "控制网页端、默认摘要输出和 Bot 推送文案。": "Sets the language for the web UI, default summaries, and Bot delivery messages.",
  "仅在网页端查看摘要": "View summaries in the web app only",
  "在这里搜索和筛选摘要记录；点开某条摘要后，会从右侧展开完整正文。": "Search and filter summary records here. Select a summary to open the full content from the right.",
  "进行中": "In progress",
} as const;

const attributeNames = ["placeholder", "title", "aria-label"] as const;

const dynamicTranslations: Array<[RegExp, (...matches: string[]) => string]> = [
  [/^已保存「(.+)」的配置。$/, (chat) => `Saved settings for "${chat}".`],
  [/^正在为「(.+)」回补 (.+) 到 (.+) 的历史消息。$/, (chat, from, to) => `Backfilling history for "${chat}" from ${from} to ${to}.`],
  [/^「(.+)」历史消息回补完成，已处理 (\d+) 条消息。$/, (chat, count) => `History backfill for "${chat}" completed. Processed ${count} messages.`],
  [/^「(.+)」历史消息回补失败：(.+)$/, (chat, error) => `History backfill for "${chat}" failed: ${error}`],
  [/^「(.+)」历史消息回补失败。$/, (chat) => `History backfill for "${chat}" failed.`],
  [/^将回补 (.+) 到 (.+) 的消息。$/, (from, to) => `Will backfill messages from ${from} to ${to}.`],
  [/^当前群类型：(.+)$/, (type) => `Current chat type: ${translateExact(type)}`],
  [/^消息 (\d+) 条 · 分块\s*(\d+)$/, (messages, chunks) => `${messages} messages · ${chunks} chunks`],
  [/^(.+) · 消息 (\d+) 条 · 分块\s*(\d+)$/, (model, messages, chunks) => `${model} · ${messages} messages · ${chunks} chunks`],
  [/^阶段提示词 1 段 · 消息 (\d+) 条 · 分块 (\d+)$/, (messages, chunks) => `1 stage prompt · ${messages} messages · ${chunks} chunks`],
  [/^字符 (\d+)$/, (count) => `${count} characters`],
  [/^消息 (\d+) 条 · 字符 (\d+)$/, (messages, chars) => `${messages} messages · ${chars} characters`],
  [/^步骤 (\d+)\/(\d+)$/, (current, total) => `Step ${current}/${total}`],
  [/^第 (\d+) \/ (\d+) 页$/, (page, total) => `Page ${page} / ${total}`],
  [/^共 (\d+) 条摘要$/, (total) => `${total} summaries`],
  [/^第 (\d+) 页，共 (\d+) 页$/, (page, total) => `Page ${page} of ${total}`],
  [/^已发送于 (.+)$/, (time) => `Sent at ${time}`],
  [/^验证码已发送到 (.+)。$/, (phone) => `Verification code sent to ${phone}.`],
  [/^Telegram 暂时限制了请求，请在 (\d+) 秒后重试。$/, (seconds) => `Telegram temporarily limited requests. Try again in ${seconds} seconds.`],
  [/^配置已保存。$/, () => "Settings saved."],
  [/^已提交摘要生成任务。$/, () => "Summary generation task submitted."],
  [/^已提交通过 Bot 发送。$/, () => "Bot delivery submitted."],
  [/^已提交重新生成。$/, () => "Regeneration submitted."],
  [/^系统配置已保存。$/, () => "System settings saved."],
  [/^访问密码至少需要 8 位。$/, () => "Access password must be at least 8 characters."],
  [/^两次输入的访问密码不一致。$/, () => "The two access passwords do not match."],
  [/^访问密码已更新。$/, () => "Access password updated."],
  [/^该账号开启了两步验证，请继续输入密码。$/, () => "This account has two-step verification enabled. Continue by entering the password."],
  [/^Telegram 登录成功。$/, () => "Telegram login succeeded."],
  [/^两步验证通过，Telegram 登录成功。$/, () => "Two-step verification passed. Telegram login succeeded."],
  [/^(.+) 已同步 (\d+) 个群组。$/, (prefix, count) => `${translateText(prefix, "en")} Synced ${count} chats.`],
  [/^(.+) 当前还没有拿到群组列表，你可以稍后重试同步。$/, (prefix) => `${translateText(prefix, "en")} No chat list is available yet. You can retry syncing later.`],
  [/^已重新同步 (\d+) 个群组。$/, (count) => `Resynced ${count} chats.`],
  [/^已重新同步，但当前没有发现可管理的群组。$/, () => "Resynced, but no manageable chats were found."],
  [/^已同步 (\d+) 个群组。$/, (count) => `Synced ${count} chats.`],
  [/^(.+) 当前没有发现可管理的群组。$/, (prefix) => `${translateText(prefix, "en")} No manageable chats were found.`],
  [/^已同步，但当前没有发现可管理的群组。$/, () => "Synced, but no manageable chats were found."],
  [/^当前已绑定：(.+)$/, (chatId) => `Currently bound: ${chatId}`],
  [/^当前将绑定：(.+)$/, (chatId) => `Will bind: ${chatId}`],
  [/^自动获取前请先完成 Telegram 登录。$/, () => "Complete Telegram login before automatic lookup."],
  [/^请先填写 Bot Token。$/, () => "Enter the Bot Token first."],
  [/^未找到最近消息，请先给 Bot 发一条消息后再重试。$/, () => "No recent message was found. Send the Bot a message first, then retry."],
  [/^已获取目标 Chat ID，点击“保存并完成”后生效。$/, () => "Target Chat ID found. It takes effect after you click Save and finish."],
  [/^找到了多个可能的会话，请选择一个。$/, () => "Multiple possible chats were found. Select one."],
  [/^已选择目标 Chat ID，点击“保存并完成”后生效。$/, () => "Target Chat ID selected. It takes effect after you click Save and finish."],
  [/^启用 Bot 推送时必须填写 Bot Token。$/, () => "Bot Token is required when Bot delivery is enabled."],
  [/^基础配置已保存，进入登录步骤。$/, () => "Basic configuration saved. Continue to login."],
  [/^配置已保存，正在进入后台。$/, () => "Settings saved. Opening dashboard."],
  [/^访问密码已设置，继续完成基础配置。$/, () => "Access password set. Continue with basic configuration."],
  [/^请填写 Telegram API ID。$/, () => "Enter Telegram API ID."],
  [/^请填写 Telegram API Hash。$/, () => "Enter Telegram API Hash."],
  [/^请填写 OpenAI API Key。$/, () => "Enter OpenAI API Key."],
  [/^请填写 Model。$/, () => "Enter Model."],
  [/^请选择有效的输出长度模式。$/, () => "Select a valid output length mode."],
  [/^自定义输出长度时，请填写有效的 Max Output Tokens。$/, () => "Enter valid Max Output Tokens when output length is custom."],
  [/^摘要并行度必须在 1 到 6 之间。$/, () => "Summary parallelism must be between 1 and 6."],
];

export function normalizeLanguage(language: string | null | undefined): Language {
  return language === "en" ? "en" : "zh-CN";
}

export function detectBrowserLanguage(): Language {
  if (typeof navigator === "undefined") {
    return "zh-CN";
  }
  return navigator.language.toLowerCase().startsWith("en") ? "en" : "zh-CN";
}

export function translateText(text: string, language: Language) {
  if (language !== "en") {
    return text;
  }
  const exact = translateExact(text);
  if (exact !== text) {
    return exact;
  }
  for (const [pattern, render] of dynamicTranslations) {
    const match = text.match(pattern);
    if (!match) {
      continue;
    }
    return render(...match.slice(1));
  }
  return text;
}

function translateExact(text: string) {
  const trimmed = text.trim();
  const translated = textTranslations[trimmed as keyof typeof textTranslations];
  if (!translated) {
    return text;
  }
  return text.replace(trimmed, translated);
}

export function I18nProvider({ children }: PropsWithChildren) {
  const [language, setLanguageState] = useState<Language>("zh-CN");

  useEffect(() => {
    setLanguageState(detectBrowserLanguage());
  }, []);

  const dict = useMemo(() => (language === "en" ? en : zh), [language]);
  const value = useMemo<I18nContextValue>(
    () => ({
      language,
      setLanguage: setLanguageState,
      dict,
    }),
    [dict, language],
  );

  return (
    <I18nContext.Provider value={value}>
      <DOMTranslator language={language} />
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return value;
}

function DOMTranslator({ language }: { language: Language }) {
  useEffect(() => {
    document.documentElement.lang = language;
    translateDocument(language);
    const observer = new MutationObserver(() => translateDocument(language));
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true,
      attributeFilter: [...attributeNames],
    });
    return () => observer.disconnect();
  }, [language]);

  return null;
}

const originalText = new WeakMap<Text, string>();
const translatedText = new WeakMap<Text, string>();

function translateDocument(language: Language) {
  translateTextNodes(document.body, language);
  translateAttributes(document.body, language);
}

function translateTextNodes(root: HTMLElement, language: Language) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let node = walker.nextNode() as Text | null;
  while (node) {
    if (shouldSkipNode(node)) {
      node = walker.nextNode() as Text | null;
      continue;
    }
    const current = node.nodeValue ?? "";
    const previousTranslated = translatedText.get(node);
    let original = originalText.get(node) ?? current;
    if (!originalText.has(node) || (previousTranslated !== undefined && current !== previousTranslated)) {
      original = current;
      originalText.set(node, original);
    }
    if (!originalText.has(node)) {
      originalText.set(node, original);
    }
    const translated = translateText(original, language);
    if (node.nodeValue !== translated) {
      node.nodeValue = translated;
    }
    translatedText.set(node, translated);
    node = walker.nextNode() as Text | null;
  }
}

function translateAttributes(root: HTMLElement, language: Language) {
  const elements = root.querySelectorAll<HTMLElement>("*");
  elements.forEach((element) => {
    if (element.closest("[data-i18n-skip='true']")) {
      return;
    }
    for (const name of attributeNames) {
      const value = element.getAttribute(name);
      if (!value) {
        continue;
      }
      const dataName = `data-i18n-original-${name}`;
      const original = element.getAttribute(dataName) ?? value;
      if (!element.hasAttribute(dataName)) {
        element.setAttribute(dataName, original);
      }
      const translated = translateText(original, language);
      if (value !== translated) {
        element.setAttribute(name, translated);
      }
    }
  });
}

function shouldSkipNode(node: Text) {
  return Boolean(node.parentElement?.closest("[data-i18n-skip='true']"));
}
