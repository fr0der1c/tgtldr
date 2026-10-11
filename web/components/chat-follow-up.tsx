"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { FollowUpConversation, followUpQuestionLimit } from "@/components/follow-up-conversation";
import { APIError, api } from "@/lib/api";
import { ChatFollowUpResponse, SummaryFollowUpTurn } from "@/lib/types";

const historyLimit = 6;
type Turn = SummaryFollowUpTurn & { note: string };

/** ChatFollowUp 依据群组截至 toDate 的历史摘要回答追问，对话只保存在当前详情中。 */
export function ChatFollowUp({ chatId, toDate }: { chatId: number; toDate: string }) {
  const [question, setQuestion] = useState("");
  const [turns, setTurns] = useState<Turn[]>([]);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const controller = useRef<AbortController | null>(null);
  const tooLong = Array.from(question).length > followUpQuestionLimit;

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
      const response = await api.followUpChat(
        chatId,
        toDate,
        submittedQuestion,
        turns.slice(-historyLimit).map(({ question, answer }) => ({ question, answer })),
        pending.signal,
      );
      if (!pending.signal.aborted) {
        setTurns((previous) => [...previous, { question: submittedQuestion, answer: response.answer, note: sourceNote(response) }]);
        setQuestion("");
      }
    } catch (error) {
      if (!pending.signal.aborted) {
        setErrorMessage(chatFollowUpError(error));
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
      turns={loading ? [...turns, { question: question.trim(), answer: null }] : turns}
      question={question}
      onQuestionChange={setQuestion}
      onSubmit={() => void handleSubmit()}
      loading={loading}
      error={errorMessage}
      description="依据该群截至这一天的历史摘要回答，从最近一天往前纳入；关闭详情后清空对话。"
    />
  );
}

function sourceNote(response: ChatFollowUpResponse) {
  return `回答依据：${response.fromDate} 至 ${response.toDate} 的 ${response.summaryCount} 条每日摘要。`;
}

function chatFollowUpError(error: unknown) {
  if (error instanceof APIError && error.code === "follow_up_context_limit") {
    return "问题或对话超出模型容量，请缩短问题或关闭详情后重新开始。";
  }
  if (error instanceof APIError && error.code === "follow_up_no_summaries") {
    return "该群在这一天及之前还没有生成成功的摘要。";
  }
  return "追问失败，请稍后重新发送。";
}
