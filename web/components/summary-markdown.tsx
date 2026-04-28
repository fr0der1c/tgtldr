"use client";

import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

export function SummaryMarkdown({ content }: { content: string }) {
  return (
    <div className="summary-markdown" data-i18n-skip="true">
      <Markdown remarkPlugins={[remarkGfm]}>{content}</Markdown>
    </div>
  );
}
