"use client";

import {
  PropsWithChildren,
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState
} from "react";
import { api } from "@/lib/api";
import { HistoryBackfillTask } from "@/lib/types";

type ToastTone = "good" | "bad" | "neutral";

type ToastItem = {
  id: number;
  message: string;
  persist: boolean;
  tone: ToastTone;
};

type ToastOptions = {
  persist?: boolean;
  tone?: ToastTone;
};

type ToastPatch = {
  message?: string;
  persist?: boolean;
  tone?: ToastTone;
};

type ToastContextValue = {
  dismiss: (id: number) => void;
  showToast: (message: string, options?: ToastOptions) => number;
  showSuccess: (message: string, options?: Omit<ToastOptions, "tone">) => number;
  showError: (message: string, options?: Omit<ToastOptions, "tone">) => number;
  showInfo: (message: string, options?: Omit<ToastOptions, "tone">) => number;
  updateToast: (id: number, patch: ToastPatch) => void;
  watchHistoryBackfill: (task: HistoryBackfillTask) => number;
};

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: PropsWithChildren) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const timeoutsRef = useRef<Map<number, number>>(new Map());
  const backfillPollersRef = useRef<Map<string, number>>(new Map());
  const backfillToastRef = useRef<Map<string, number>>(new Map());

  const dismiss = useCallback((id: number) => {
    const timeoutID = timeoutsRef.current.get(id);
    if (timeoutID) {
      window.clearTimeout(timeoutID);
      timeoutsRef.current.delete(id);
    }
    for (const [taskID, toastID] of backfillToastRef.current.entries()) {
      if (toastID !== id) {
        continue;
      }
      const pollerID = backfillPollersRef.current.get(taskID);
      if (pollerID) {
        window.clearInterval(pollerID);
        backfillPollersRef.current.delete(taskID);
      }
      backfillToastRef.current.delete(taskID);
    }
    setToasts((current) => current.filter((item) => item.id !== id));
  }, []);

  const scheduleDismiss = useCallback(
    (id: number) => {
      const currentTimeout = timeoutsRef.current.get(id);
      if (currentTimeout) {
        window.clearTimeout(currentTimeout);
      }
      const timeoutID = window.setTimeout(() => dismiss(id), 3200);
      timeoutsRef.current.set(id, timeoutID);
    },
    [dismiss]
  );

  const showToast = useCallback(
    (message: string, options?: ToastOptions) => {
      const id = Date.now() + Math.floor(Math.random() * 1000);
      const persist = options?.persist ?? false;
      const tone = options?.tone ?? "good";
      setToasts((current) => [...current, { id, message, persist, tone }]);
      if (!persist) {
        scheduleDismiss(id);
      }
      return id;
    },
    [scheduleDismiss]
  );

  const updateToast = useCallback(
    (id: number, patch: ToastPatch) => {
      setToasts((current) =>
        current.map((item) => {
          if (item.id !== id) {
            return item;
          }
          return {
            ...item,
            message: patch.message ?? item.message,
            persist: patch.persist ?? item.persist,
            tone: patch.tone ?? item.tone
          };
        })
      );

      const persist = patch.persist ?? toasts.find((item) => item.id === id)?.persist ?? false;
      if (persist) {
        const timeoutID = timeoutsRef.current.get(id);
        if (timeoutID) {
          window.clearTimeout(timeoutID);
          timeoutsRef.current.delete(id);
        }
        return;
      }
      scheduleDismiss(id);
    },
    [scheduleDismiss, toasts]
  );

  const watchHistoryBackfill = useCallback(
    (task: HistoryBackfillTask) => {
      const existingToastID = backfillToastRef.current.get(task.id);
      if (existingToastID) {
        return existingToastID;
      }

      const toastID = showToast(
        `正在为「${task.chatTitle}」回补 ${task.fromDate} 到 ${task.toDate} 的历史消息。`,
        { tone: "neutral", persist: true }
      );
      backfillToastRef.current.set(task.id, toastID);

      const pollerID = window.setInterval(async () => {
        try {
          const latest = await api.getHistoryBackfill(task.id);
          if (latest.status === "pending" || latest.status === "running") {
            updateToast(toastID, {
              message:
                latest.errorMessage.trim() !== ""
                  ? `「${latest.chatTitle}」${latest.errorMessage}`
                  : `正在为「${latest.chatTitle}」回补 ${latest.fromDate} 到 ${latest.toDate} 的历史消息。`,
              persist: true,
              tone: "neutral"
            });
            return;
          }

          window.clearInterval(pollerID);
          backfillPollersRef.current.delete(task.id);
          backfillToastRef.current.delete(task.id);

          if (latest.status === "succeeded") {
            updateToast(toastID, {
              message: `「${latest.chatTitle}」历史消息回补完成，已处理 ${latest.importedCount} 条消息。`,
              persist: true,
              tone: "good"
            });
            return;
          }

          updateToast(toastID, {
            message: latest.errorMessage
              ? `「${latest.chatTitle}」历史消息回补失败：${latest.errorMessage}`
              : `「${latest.chatTitle}」历史消息回补失败。`,
            persist: true,
            tone: "bad"
          });
        } catch (error) {
          window.clearInterval(pollerID);
          backfillPollersRef.current.delete(task.id);
          backfillToastRef.current.delete(task.id);
          updateToast(toastID, {
            message: error instanceof Error ? error.message : String(error),
            persist: true,
            tone: "bad"
          });
        }
      }, 3000);

      backfillPollersRef.current.set(task.id, pollerID);
      return toastID;
    },
    [showToast, updateToast]
  );

  const value = useMemo<ToastContextValue>(
    () => ({
      dismiss,
      showToast,
      showSuccess: (message, options) => showToast(message, { ...options, tone: "good" }),
      showError: (message, options) => showToast(message, { ...options, tone: "bad" }),
      showInfo: (message, options) => showToast(message, { ...options, tone: "neutral" }),
      updateToast,
      watchHistoryBackfill
    }),
    [dismiss, showToast, updateToast, watchHistoryBackfill]
  );

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-viewport" aria-live="polite" aria-atomic="true">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast ${toast.tone}`} role="status">
            <span>{toast.message}</span>
            <button
              aria-label="关闭提示"
              className="toast-close"
              onClick={() => dismiss(toast.id)}
              type="button"
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used within ToastProvider");
  }
  return context;
}
