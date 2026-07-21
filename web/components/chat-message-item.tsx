"use client";

import { CSSProperties, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { ChatMessage, ChatMessageMedia, ChatMessageReply } from "@/lib/types";
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
  const text = visibleMessageText(message);
  const mediaLabel = visibleMediaLabel(message.mediaKind, message.messageType, language);
	const [media, setMedia] = useState(message.media);

  return (
    <article className={`chat-message ${highlighted ? "highlighted" : ""}`} data-message-id={message.id}>
      {message.senderAvatarUrl ? (
        <img alt="" className="chat-message-avatar image" src={message.senderAvatarUrl} />
      ) : (
        <div className="chat-message-avatar" style={avatarStyle(sender)}>
          {sender.slice(0, 1).toUpperCase()}
        </div>
      )}
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
        {media ? (
          <MediaAttachment language={language} media={media} onQueued={() => setMedia({ ...media, status: "pending", canDownload: false, canRetry: false })} />
        ) : mediaLabel ? (
          <div className="chat-message-media-label">
            <MediaIcon />
            <span>{mediaLabel}</span>
          </div>
        ) : null}
        {text ? <p className="chat-message-text">{text}</p> : null}
      </div>
    </article>
  );
}

// 展示资源下载状态，并提供手动下载、超限确认或失败重试入口。
function MediaAttachment({
  media,
  language,
  onQueued,
}: {
  media: ChatMessageMedia;
  language: "zh-CN" | "en";
  onQueued: () => void;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  // 将策略暂停、超限或失败资源重新提交给后台下载队列。
  async function queueDownload() {
    setSubmitting(true);
    setError("");
    try {
      await api.downloadAsset(media.id);
      onQueued();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setSubmitting(false);
    }
  }

  if (media.status === "succeeded" && media.contentUrl) {
    return <DownloadedMedia language={language} media={media} />;
  }

  const waiting = media.status === "pending" || media.status === "downloading";
  const sticker = media.kind === "sticker";
  const label = waiting
    ? sticker
      ? (language === "en" ? "Downloading sticker…" : "正在下载贴纸…")
      : (language === "en" ? "Downloading media…" : "正在下载媒体…")
    : media.status === "manual"
      ? sticker
        ? (language === "en" ? "Sticker is waiting for manual download" : "贴纸等待手动下载")
        : (language === "en" ? "Attachment is waiting for manual download" : "附件等待手动下载")
      : media.status === "skipped_oversize"
        ? (language === "en" ? "File exceeds the 100 MB automatic download limit" : "文件超过 100 MB 自动下载上限")
        : sticker
          ? (language === "en" ? "Sticker download failed" : "贴纸下载失败")
          : (language === "en" ? "Media download failed" : "媒体下载失败");
  return (
    <div className="chat-message-media-state">
      <div><MediaIcon /><span>{label}</span></div>
      {!waiting ? (
        <button disabled={submitting} onClick={() => void queueDownload()} type="button">
          {submitting
            ? (language === "en" ? "Queuing…" : "正在提交…")
            : media.status === "manual"
              ? sticker
                ? (language === "en" ? "Download sticker" : "下载贴纸")
                : (language === "en" ? "Download attachment" : "下载附件")
              : media.canDownload
                ? (language === "en" ? "Download anyway" : "仍要下载")
                : (language === "en" ? "Retry" : "重试")}
        </button>
      ) : null}
      {error ? <small>{error}</small> : null}
    </div>
  );
}

// 根据媒体类型选择浏览器原生预览控件或附件卡片。
function DownloadedMedia({ media, language }: { media: ChatMessageMedia; language: "zh-CN" | "en" }) {
  if (media.kind === "sticker") {
    if (media.mimeType === "application/x-tgsticker") {
      return <AnimatedSticker contentUrl={media.contentUrl!} language={language} />;
    }
    if (media.mimeType === "video/webm") {
      return <video autoPlay className="chat-message-sticker" loop muted playsInline src={media.contentUrl} />;
    }
    return <img alt={media.fileName} className="chat-message-sticker" loading="lazy" src={media.contentUrl} />;
  }
  if (media.kind === "photo") {
    return <a href={media.contentUrl} rel="noreferrer" target="_blank"><img alt={media.fileName} className="chat-message-photo" loading="lazy" src={media.contentUrl} /></a>;
  }
  if (media.kind === "video") {
    return <video className="chat-message-video" controls preload="metadata" src={media.contentUrl} />;
  }
  if (media.kind === "audio" || media.kind === "voice") {
    return <audio className="chat-message-audio" controls preload="metadata" src={media.contentUrl} />;
  }
  return (
    <a className="chat-message-file" download href={media.contentUrl}>
      <MediaIcon />
      <span><strong>{media.fileName}</strong><small>{formatFileSize(media.size, language)}</small></span>
    </a>
  );
}

// 从受保护资源接口加载解压后的 TGS JSON，并在组件卸载时释放动画实例。
function AnimatedSticker({ contentUrl, language }: { contentUrl: string; language: "zh-CN" | "en" }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    let animation: { destroy: () => void } | undefined;

    async function load() {
      try {
        const [response, module] = await Promise.all([
          fetch(contentUrl, { credentials: "same-origin", signal: controller.signal }),
          import("lottie-web"),
        ]);
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }
        const container = containerRef.current;
        if (!container) {
          return;
        }
        animation = module.default.loadAnimation({
          container,
          renderer: "svg",
          loop: true,
          autoplay: true,
          animationData: await response.json(),
        });
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setFailed(true);
        }
      }
    }

    void load();
    return () => {
      controller.abort();
      animation?.destroy();
    };
  }, [contentUrl]);

  if (failed) {
    return <div className="chat-message-sticker-error">{language === "en" ? "Sticker could not be displayed" : "贴纸无法显示"}</div>;
  }
  return <div className="chat-message-sticker" ref={containerRef} />;
}

// 使用当前界面语言格式化文件大小。
function formatFileSize(size: number, language: "zh-CN" | "en") {
  if (!size) return language === "en" ? "Unknown size" : "未知大小";
  return new Intl.NumberFormat(language, { style: "unit", unit: size >= 1024 * 1024 ? "megabyte" : "kilobyte", maximumFractionDigits: 1 })
    .format(size / (size >= 1024 * 1024 ? 1024 * 1024 : 1024));
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
  const text = visibleMessageText(reply);
  return (
    <div className="chat-message-reply">
      <strong>{reply.senderName || "未知发言人"}</strong>
      {text ? <span>{text}</span> : null}
    </div>
  );
}

function visibleMessageText(
  message: Pick<ChatMessage, "textContent" | "caption" | "mediaKind" | "messageType">,
) {
  const text = message.textContent.trim() || message.caption.trim();
  return text;
}

function visibleMediaLabel(mediaKind: string, messageType: string, language: "zh-CN" | "en") {
  if (mediaKind === "photo") return language === "en" ? "Photo" : "图片消息";
  if (mediaKind === "sticker") return language === "en" ? "Sticker" : "贴纸";
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
