"use client";

import { BotTargetChatCandidate } from "@/lib/types";

export function hasAvailableBotToken(current?: string, placeholder = "") {
  return (current ?? "").trim() !== "" || placeholder.trim() !== "";
}

export function describeBotChatCandidate(candidate: BotTargetChatCandidate) {
  const typeLabel = botChatTypeLabel(candidate.chatType);
  const username = candidate.username?.trim() ? `@${candidate.username.trim()}` : "";
  const primary = candidate.title?.trim() || username || candidate.chatId;
  const suffix = username && primary !== username ? ` (${username})` : "";
  return `${typeLabel} · ${primary}${suffix}`;
}

function botChatTypeLabel(chatType: string) {
  switch (chatType.trim()) {
    case "private":
      return "私聊";
    case "group":
      return "群聊";
    case "supergroup":
      return "超级群";
    case "channel":
      return "频道";
    default:
      return "会话";
  }
}
