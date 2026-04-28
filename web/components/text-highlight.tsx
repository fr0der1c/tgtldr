"use client";

import { Fragment } from "react";

export function TextHighlight({
  text,
  terms
}: {
  text: string;
  terms: string[];
}) {
  const tokens = tokenizeHighlights(text, terms);

  return (
    <span data-i18n-skip="true">
      {tokens.map((token, index) =>
        token.highlight ? (
          <mark className="search-highlight" key={`${token.value}-${index}`}>
            {token.value}
          </mark>
        ) : (
          <Fragment key={`${token.value}-${index}`}>{token.value}</Fragment>
        )
      )}
    </span>
  );
}

type HighlightToken = {
  value: string;
  highlight: boolean;
};

function tokenizeHighlights(text: string, terms: string[]): HighlightToken[] {
  if (!text) {
    return [{ value: "", highlight: false }];
  }

  const normalizedTerms = normalizeTerms(terms);
  if (normalizedTerms.length === 0) {
    return [{ value: text, highlight: false }];
  }

  const ranges = buildRanges(text, normalizedTerms);
  if (ranges.length === 0) {
    return [{ value: text, highlight: false }];
  }

  const tokens: HighlightToken[] = [];
  let cursor = 0;
  for (const range of ranges) {
    if (range.start > cursor) {
      tokens.push({ value: text.slice(cursor, range.start), highlight: false });
    }
    tokens.push({ value: text.slice(range.start, range.end), highlight: true });
    cursor = range.end;
  }
  if (cursor < text.length) {
    tokens.push({ value: text.slice(cursor), highlight: false });
  }
  return tokens;
}

function normalizeTerms(terms: string[]): string[] {
  const unique = new Set<string>();
  for (const term of terms) {
    const value = term.trim();
    if (!value) {
      continue;
    }
    unique.add(value.toLowerCase());
  }
  return Array.from(unique).sort((left, right) => right.length - left.length);
}

function buildRanges(text: string, terms: string[]) {
  const ranges: Array<{ start: number; end: number }> = [];
  const lower = text.toLowerCase();

  for (const term of terms) {
    let searchFrom = 0;
    while (searchFrom < lower.length) {
      const index = lower.indexOf(term, searchFrom);
      if (index === -1) {
        break;
      }
      ranges.push({ start: index, end: index + term.length });
      searchFrom = index + term.length;
    }
  }

  if (ranges.length === 0) {
    return ranges;
  }

  ranges.sort((left, right) => left.start - right.start || right.end - left.end);
  const merged = [ranges[0]];
  for (const range of ranges.slice(1)) {
    const last = merged[merged.length - 1];
    if (range.start > last.end) {
      merged.push(range);
      continue;
    }
    if (range.end > last.end) {
      last.end = range.end;
    }
  }
  return merged;
}
