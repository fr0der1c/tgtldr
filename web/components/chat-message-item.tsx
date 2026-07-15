import { CSSProperties } from "react";
import { ChatMessage, ChatMessageReply } from "@/lib/types";
import { StatusPill } from "@/components/ui";

type ChatMessageItemProps = {
  highlighted?: boolean;
  message: ChatMessage;
  language: "zh-CN" | "en";
  timezone: string;
};

export function ChatMessageItem({
  highlighted = false,
  message,
  language,
  timezone,
}: ChatMessageItemProps) {
  const sender = message.senderName.trim() || "未知发言人";
  const text = visibleMessageText(message, language);
  const mediaLabel = visibleMediaLabel(message.mediaKind, message.messageType, language);

  return (
    <article className={`chat-message ${highlighted ? "highlighted" : ""}`} data-message-id={message.id}>
      <div className="chat-message-avatar" style={avatarStyle(sender)}>
        {sender.slice(0, 1).toUpperCase()}
      </div>
      <div className="chat-message-content">
        <header className="chat-message-meta">
          <strong>{sender}</strong>
          {message.senderUsername ? <span>@{message.senderUsername}</span> : null}
          {message.senderIsBot ? <StatusPill tone="neutral">Bot</StatusPill> : null}
          <time dateTime={message.messageTime}>
            {formatMessageTime(message.messageTime, timezone, language)}
          </time>
        </header>
        {message.reply ? (
          <ReplyPreview language={language} reply={message.reply} />
        ) : null}
        {mediaLabel ? (
          <div className="chat-message-media-label">
            <MediaIcon />
            <span>{mediaLabel}</span>
          </div>
        ) : null}
        <p className={`chat-message-text ${text.placeholder ? "placeholder" : ""}`}>
          {text.value}
        </p>
      </div>
    </article>
  );
}

function ReplyPreview({
  reply,
  language,
}: {
  reply: ChatMessageReply;
  language: "zh-CN" | "en";
}) {
  if (!reply.found) {
    return (
      <div className="chat-message-reply missing">
        <strong>原消息未找到</strong>
        <span>#{reply.telegramMessageId}</span>
      </div>
    );
  }
  const text = visibleMessageText(reply, language);
  return (
    <div className="chat-message-reply">
      <strong>{reply.senderName || "未知发言人"}</strong>
      <span>{text.value}</span>
    </div>
  );
}

function visibleMessageText(
  message: Pick<ChatMessage, "textContent" | "caption" | "mediaKind" | "messageType">,
  language: "zh-CN" | "en",
) {
  const text = message.textContent.trim() || message.caption.trim();
  if (text) {
    return { value: text, placeholder: false };
  }
  const media = visibleMediaLabel(message.mediaKind, message.messageType, language);
  if (language === "en") {
    return { value: media ? `${media} without a caption` : "Non-text message without a caption", placeholder: true };
  }
  return { value: media ? `${media}，无文字说明` : "非文本消息，无文字说明", placeholder: true };
}

function visibleMediaLabel(mediaKind: string, messageType: string, language: "zh-CN" | "en") {
  if (mediaKind === "photo") return language === "en" ? "Photo" : "图片消息";
  if (mediaKind === "document") return language === "en" ? "File" : "文件消息";
  if (mediaKind) return language === "en" ? "Media" : "媒体消息";
  if (messageType && messageType !== "text") return language === "en" ? "Non-text message" : "非文本消息";
  return "";
}

function formatMessageTime(value: string, timezone: string, language: "zh-CN" | "en") {
  return new Intl.DateTimeFormat(language, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: timezone,
  }).format(new Date(value));
}

function avatarStyle(sender: string) {
  let hash = 0;
  for (const character of sender) {
    hash = (hash * 31 + character.charCodeAt(0)) % 360;
  }
  return { "--avatar-hue": hash } as CSSProperties;
}

function MediaIcon() {
  return (
    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24">
      <path d="M5 4.5h9l5 5V19.5H5v-15Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.7" />
      <path d="M14 4.5v5h5M8.5 14h7M8.5 17h5" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </svg>
  );
}
