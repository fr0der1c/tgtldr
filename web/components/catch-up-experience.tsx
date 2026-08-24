"use client";

import { useCallback, useState } from "react";
import { CatchUpDrawer } from "@/components/catch-up-drawer";
import { CatchUpWizard } from "@/components/catch-up-wizard";
import { Button } from "@/components/ui";
import { useToast } from "@/components/toast";
import { CatchUp, Chat, Summary } from "@/lib/types";

/** CatchUpExperience 协调摘要页 CTA、生成向导和历史详情侧栏。 */
export function CatchUpExperience({
  botReady,
  chats,
  onOpenSummary,
  timezone,
}: {
  botReady: boolean;
  chats: Chat[];
  onOpenSummary: (summary: Summary) => void;
  timezone: string;
}) {
  const [wizardOpen, setWizardOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerTargetId, setDrawerTargetId] = useState<number | null>(null);
  const toast = useToast();

  const openHistory = useCallback(() => {
    setWizardOpen(false);
    setDrawerTargetId(null);
    setDrawerOpen(true);
  }, []);

  /** 等待中的任务完成后直接打开详情；后台完成则只给出轻量通知。 */
  const handleCompleted = useCallback((item: CatchUp, waitedInDialog: boolean) => {
    if (waitedInDialog) {
      setWizardOpen(false);
      setDrawerTargetId(item.id);
      setDrawerOpen(true);
      return;
    }
    if (item.status === "succeeded") toast.showSuccess("快速回顾已生成完成。");
    else toast.showError(item.errorMessage || "快速回顾生成失败。");
  }, [toast]);

  return (
    <>
      <section className="catch-up-cta">
        <div>
          <span>快速回顾</span>
          <h2>很久没看摘要了吗？开始快速回顾一下吧</h2>
          <p>选择一段时间和群组，把散落的每日摘要整理成一份阶段回顾。</p>
        </div>
        <Button disabled={chats.length === 0} onClick={() => setWizardOpen(true)} type="button">
          开始快速回顾
        </Button>
      </section>

      <CatchUpWizard
        botReady={botReady}
        chats={chats}
        onClose={() => setWizardOpen(false)}
        onCompleted={handleCompleted}
        onOpenHistory={openHistory}
        open={wizardOpen}
        timezone={timezone}
      />

      <CatchUpDrawer
        initialCatchUpId={drawerTargetId}
        onClose={() => setDrawerOpen(false)}
        onOpenSummary={onOpenSummary}
        open={drawerOpen}
      />
    </>
  );
}
