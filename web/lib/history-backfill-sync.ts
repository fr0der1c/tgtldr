import { HistoryBackfillTask } from "@/lib/types";

const HISTORY_BACKFILL_COMPLETED_EVENT = "tgtldr:history-backfill-completed";

export type HistoryBackfillCompletedDetail = Pick<
  HistoryBackfillTask,
  "chatId" | "id" | "importedCount" | "status"
>;

export function notifyHistoryBackfillCompleted(task: HistoryBackfillTask) {
  if (typeof window === "undefined") {
    return;
  }

  window.dispatchEvent(
    new CustomEvent<HistoryBackfillCompletedDetail>(HISTORY_BACKFILL_COMPLETED_EVENT, {
      detail: {
        chatId: task.chatId,
        id: task.id,
        importedCount: task.importedCount,
        status: task.status
      }
    })
  );
}

export function onHistoryBackfillCompleted(
  listener: (detail: HistoryBackfillCompletedDetail) => void
) {
  if (typeof window === "undefined") {
    return () => undefined;
  }

  const handler = (event: Event) => {
    listener((event as CustomEvent<HistoryBackfillCompletedDetail>).detail);
  };
  window.addEventListener(HISTORY_BACKFILL_COMPLETED_EVENT, handler);
  return () => window.removeEventListener(HISTORY_BACKFILL_COMPLETED_EVENT, handler);
}
