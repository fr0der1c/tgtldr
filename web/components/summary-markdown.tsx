"use client";

import { useMemo } from "react";
import Markdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

export type MarkdownSourceReference = {
  label: string;
  title: string;
  ariaLabel: string;
};

const emptySourceReferences = new Map<string, MarkdownSourceReference>();

/** SummaryMarkdown 可选地把已知来源编号渲染为可点击的行内引用。 */
export function SummaryMarkdown({
  content,
  sourceReferences = emptySourceReferences,
  onSourceReferenceClick,
}: {
  content: string;
  sourceReferences?: ReadonlyMap<string, MarkdownSourceReference>;
  onSourceReferenceClick?: (reference: string) => void;
}) {
  const renderedContent = useMemo(
    () => onSourceReferenceClick
      ? linkSourceReferences(content, sourceReferences)
      : content,
    [content, onSourceReferenceClick, sourceReferences],
  );
  const components = useMemo<Components>(() => ({
    a({ node: _node, href, children, ...props }) {
      const reference = sourceReferenceFromHref(href);
      const source = reference ? sourceReferences.get(reference) : undefined;
      if (reference && source && onSourceReferenceClick) {
        return (
          <button
            aria-label={source.ariaLabel}
            className="summary-source-reference"
            onClick={() => onSourceReferenceClick(reference)}
            title={source.title}
            type="button"
          >
            [{source.label}]
          </button>
        );
      }
      return <a {...props} href={href}>{children}</a>;
    },
  }), [onSourceReferenceClick, sourceReferences]);

  return (
    <div className="summary-markdown" data-i18n-skip="true">
      <Markdown components={components} remarkPlugins={[remarkGfm]}>{renderedContent}</Markdown>
    </div>
  );
}

/** linkSourceReferences 只转换来源表中存在且尚未被写成 Markdown 链接的标识。 */
function linkSourceReferences(content: string, references: ReadonlyMap<string, MarkdownSourceReference>) {
  if (references.size === 0) return content;

  let insideFence = false;
  return content.split("\n").map((line) => {
    if (/^\s*(```|~~~)/.test(line)) {
      insideFence = !insideFence;
      return line;
    }
    if (insideFence) return line;
    return linkSourceReferencesInLine(line, references);
  }).join("\n");
}

/** linkSourceReferencesInLine 跳过行内代码，只替换普通 Markdown 文本。 */
function linkSourceReferencesInLine(line: string, references: ReadonlyMap<string, MarkdownSourceReference>) {
  return line.split(/(`[^`]*`)/g).map((part, index) => {
    if (index % 2 === 1) return part;
    return part.replace(/\[(S\d{3,})\](?!\()/g, (match, reference: string) => {
      const source = references.get(reference);
      return source ? `[${source.label}](#catch-up-source-${reference})` : match;
    });
  }).join("");
}

function sourceReferenceFromHref(href: string | undefined) {
  return href?.match(/^#catch-up-source-(S\d{3,})$/)?.[1];
}
