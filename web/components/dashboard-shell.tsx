"use client";

import { PropsWithChildren, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Bootstrap } from "@/lib/types";
import { onBootstrapRefresh } from "@/lib/bootstrap-sync";
import { NavLink, StatusPill } from "@/components/ui";
import { useI18n } from "@/lib/i18n";

export function DashboardShell({ children }: PropsWithChildren) {
  const router = useRouter();
  const { setLanguage } = useI18n();
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);

  useEffect(() => {
    function refreshBootstrap() {
      void api
        .bootstrap()
        .then((data) => {
          setBootstrap(data);
          setLanguage(data.language);
          if (data.passwordConfigured && !data.authenticated) {
            router.replace("/login");
          }
        })
        .catch(() => null);
    }

    refreshBootstrap();
    return onBootstrapRefresh(refreshBootstrap);
  }, [router, setLanguage]);

  return (
    <div className="dashboard-layout">
      <aside className="dashboard-sidebar">
        <div className="dashboard-brand">
          <p className="dashboard-brand-mark">TGTLDR</p>
          <p className="dashboard-brand-copy">
            Too long, don't read. 为你每天节省时间。
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
        </div>
      </aside>
      <div className="dashboard-main">{children}</div>
    </div>
  );
}
