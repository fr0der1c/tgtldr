"use client";

import { useEffect, useId, useRef } from "react";
import { SummaryMarkdown } from "@/components/summary-markdown";
import { Button, Textarea } from "@/components/ui";
import { translateText, useI18n } from "@/lib/i18n";

export const followUpQuestionLimit = 2000;

type Turn = { question: string; answer: string | null; source?: "messages" | "summary" };

export function FollowUpConversation({ turns, question, onQuestionChange, onSubmit, loading, error, description, title }: {
  turns: Turn[];
  question: string;
  onQuestionChange: (value: string) => void;
  onSubmit: () => void;
  loading: boolean;
  error: string | null;
  description: string;
  title?: string;
}) {
  const { language } = useI18n();
  const t = (text: string) => translateText(text, language);
  const inputId = useId();
  const threadRef = useRef<HTMLDivElement>(null);
  const count = Array.from(question).length;
  const tooLong = count > followUpQuestionLimit;

  useEffect(() => {
    if (threadRef.current) threadRef.current.scrollTop = threadRef.current.scrollHeight;
  }, [turns, loading]);

  return (
    <form className="followup-box" onSubmit={(event) => { event.preventDefault(); if (!loading && !tooLong && question.trim()) onSubmit(); }}>
      {title ? <h3>{t(title)}</h3> : null}
      <p className="followup-label">{t(description)}</p>
      {turns.length > 0 ? (
        <div className="followup-thread" ref={threadRef} aria-live="polite">
          {turns.map((turn, index) => (
            <div className="followup-pair" key={index}>
              <div className="followup-question">{turn.question}</div>
              <div className="followup-answer">
                {turn.answer === null ? <span className="followup-thinking" role="status">{t("正在回答…")}</span> : (
                  <>
                    {turn.source ? <p className="muted">{t(turn.source === "messages"
                      ? "回答依据：当天原始消息。"
                      : "回答依据：摘要。原始消息不可用或超出模型容量，细节可能不完整。")}</p> : null}
                    <SummaryMarkdown content={turn.answer} />
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      ) : null}
      {error ? <p className="followup-error" role="alert">{t(error)}</p> : null}
      <label className="followup-label" htmlFor={inputId}>{t("你的问题")}</label>
      <div className="followup-input-row">
        <Textarea id={inputId} disabled={loading} rows={2} value={question}
          aria-invalid={tooLong || undefined}
          aria-describedby={`${inputId}-limit`}
          placeholder={t("输入问题，按 Enter 发送（Shift+Enter 换行）")}
          onChange={(event) => onQuestionChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing && event.keyCode !== 229) {
              event.preventDefault();
              if (!loading && !tooLong && question.trim()) onSubmit();
            }
          }} />
        <Button disabled={loading || tooLong || !question.trim()} type="submit">{t(loading ? "正在回答…" : "发送问题")}</Button>
      </div>
      <span className="followup-label" id={`${inputId}-limit`}>{count} / {followUpQuestionLimit} · {t("最多 2000 个字符")}</span>
      {tooLong ? <p className="followup-error" role="alert">{t("问题不能超过 2000 个字符。")}</p> : null}
    </form>
  );
}
