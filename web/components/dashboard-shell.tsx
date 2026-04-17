"use client";

import { PropsWithChildren, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Bootstrap } from "@/lib/types";
import { NavLink, StatusPill } from "@/components/ui";

export function DashboardShell({ children }: PropsWithChildren) {
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);

  useEffect(() => {
    void api.bootstrap().then(setBootstrap).catch(() => null);
  }, []);

  return (
    <div className="dashboard-layout">
      <aside className="dashboard-sidebar">
        <div className="dashboard-brand">
          <p className="dashboard-brand-mark">TGTLDR</p>
          <p className="dashboard-brand-copy">
            单用户自部署的 Telegram 群摘要工作台。
          </p>
        </div>

        <nav className="nav-stack">
          <NavLink href="/dashboard/chats">群组</NavLink>
          <NavLink href="/dashboard/summaries">摘要</NavLink>
          <NavLink href="/dashboard/settings">系统配置</NavLink>
        </nav>

        <div className="dashboard-sidebar-status">
          <div className="sidebar-status-item">
            <span>Telegram</span>
            <StatusPill tone={bootstrap?.telegramAuthorized ? "good" : "warn"}>
              {bootstrap?.telegramAuthorized ? "已连接" : "未连接"}
            </StatusPill>
          </div>
          <div className="sidebar-status-item">
            <span>Bot 推送</span>
            <StatusPill tone={bootstrap?.botEnabled ? "good" : "neutral"}>
              {bootstrap?.botEnabled ? "启用中" : "未启用"}
            </StatusPill>
          </div>
          <div className="sidebar-status-item">
            <span>已启用消息保存</span>
            <strong>{bootstrap?.enabledChatCount ?? 0}</strong>
          </div>
        </div>
      </aside>
      <div className="dashboard-main">{children}</div>
    </div>
  );
}
