"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { FollowUpConversation, followUpQuestionLimit } from "@/components/follow-up-conversation";
import { APIError, api } from "@/lib/api";
import { SummaryFollowUpResponse, SummaryFollowUpTurn } from "@/lib/types";

const historyLimit = 6;
type Turn = SummaryFollowUpTurn & Pick<SummaryFollowUpResponse, "source">;

export function SummaryFollowUp({ summaryId }: { summaryId: number }) {
  const [question, setQuestion] = useState("");
  const [turns, setTurns] = useState<Turn[]>([]);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const controller = useRef<AbortController | null>(null);
  const characterCount = Array.from(question).length;
  const tooLong = characterCount > followUpQuestionLimit;

  useLayoutEffect(() => () => controller.current?.abort(), []);

  async function handleSubmit() {
    const submittedQuestion = question.trim();
    if (!submittedQuestion || tooLong || controller.current) {
      return;
    }
    const pending = new AbortController();
    controller.current = pending;
    setLoading(true);
    setErrorMessage("");
    try {
      const response = await api.followUpSummary(
        summaryId,
        submittedQuestion,
        turns.slice(-historyLimit).map(({ question, answer }) => ({ question, answer })),
        pending.signal,
      );
      if (!pending.signal.aborted) {
        setTurns((previous) => [...previous, { question: submittedQuestion, ...response }]);
        setQuestion("");
      }
    } catch (error) {
      if (!pending.signal.aborted) {
        setErrorMessage(error instanceof APIError && error.code === "follow_up_context_limit"
          ? "问题或对话超出模型容量，请缩短问题或关闭详情后重新开始。"
          : "追问失败，请稍后重新发送。");
      }
    } finally {
      if (!pending.signal.aborted) {
        controller.current = null;
        setLoading(false);
      }
    }
  }

  return (
    <FollowUpConversation
      title="追问这条摘要"
      turns={loading ? [...turns, { question: question.trim(), answer: null }] : turns}
      question={question}
      onQuestionChange={setQuestion}
      onSubmit={() => void handleSubmit()}
      loading={loading}
      error={errorMessage}
      description="仅针对当天内容追问；关闭详情后清空对话。"
    />
  );
}
