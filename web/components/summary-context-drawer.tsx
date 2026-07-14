"use client";

import type { PropsWithChildren } from "react";
import { Drawer } from "@/components/drawer";
import { EmptyState } from "@/components/dashboard-page";
import { SummaryContextPreview } from "@/lib/types";

export function SummaryContextDrawer({
  open,
  loading,
  preview,
  onClose
}: {
  open: boolean;
  loading: boolean;
  preview: SummaryContextPreview | null;
  onClose: () => void;
}) {
  const chunks = Array.isArray(preview?.chunks) ? preview.chunks : [];

  return (
    <Drawer layer="top" onClose={onClose} open={open} title="原始 prompt">
      {loading ? (
        <p className="muted">正在加载上下文预览…</p>
      ) : !preview ? (
        <EmptyState
          description="稍后重试，或先确认这条摘要对应的消息仍然存在。"
          title="暂时无法生成上下文预览"
        />
      ) : (
        <div className="summary-context-stack">
          <p className="summary-context-drawer-meta">
            阶段提示词 1 段 · 消息 {preview.messageCount} 条 · 分块 {preview.chunkCount}
          </p>
          <ContextDisclosure
            meta={`字符 ${preview.systemPrompt.length}`}
            title="系统提示词"
          >
            <pre className="summary-context-block">{preview.systemPrompt}</pre>
          </ContextDisclosure>

          {chunks.map((chunk) => (
            <ContextDisclosure
              key={chunk.index}
              meta={`消息 ${chunk.messageCount} 条 · 字符 ${chunk.content.length}`}
              title={`Chunk ${chunk.index + 1}`}
            >
              <pre className="summary-context-block">{chunk.content}</pre>
            </ContextDisclosure>
          ))}

          {preview.finalPrompt ? (
            <ContextDisclosure meta="最终汇总阶段" title="合并提示词">
              <pre className="summary-context-block">{preview.finalPrompt}</pre>
              {preview.finalInputNotice ? (
                <p className="muted">{preview.finalInputNotice}</p>
              ) : null}
            </ContextDisclosure>
          ) : null}

          <p className="muted">{preview.previewNotice}</p>
        </div>
      )}
    </Drawer>
  );
}

function ContextDisclosure({
  title,
  meta,
  children
}: PropsWithChildren<{
  title: string;
  meta: string;
}>) {
  return (
    <details className="summary-context-disclosure">
      <summary className="summary-context-summary">
        <div className="summary-context-summary-copy">
          <strong>{title}</strong>
          <span>{meta}</span>
        </div>
        <span aria-hidden="true" className="summary-context-summary-icon">
          ▾
        </span>
      </summary>
      <div className="summary-context-panel">{children}</div>
    </details>
  );
}
