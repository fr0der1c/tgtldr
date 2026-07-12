"use client";

import Link from "next/link";
import { CSSProperties, PropsWithChildren, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Bootstrap } from "@/lib/types";
import { onBootstrapRefresh } from "@/lib/bootstrap-sync";
import { StatusPill } from "@/components/ui";
import { useI18n } from "@/lib/i18n";

export function DashboardShell({ children }: PropsWithChildren) {
  const router = useRouter();
  const pathname = usePathname();
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

        <nav
          className="nav-stack"
          style={{ "--active-nav-index": activeNavigationIndex(pathname) } as CSSProperties}
        >
          <span aria-hidden="true" className="nav-active-indicator" />
          {dashboardNavigation.map((item) => (
            <Link
              className={`nav-link ${pathname === item.href ? "active" : ""}`}
              href={item.href}
              key={item.href}
            >
              {item.label}
            </Link>
          ))}
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
      <div className="dashboard-main">
        <div className="dashboard-main-content" key={pathname}>
          {children}
        </div>
      </div>
    </div>
  );
}

const dashboardNavigation = [
  { href: "/dashboard/chats", label: "群组" },
  { href: "/dashboard/summaries", label: "摘要" },
  { href: "/dashboard/settings", label: "系统配置" },
];

function activeNavigationIndex(pathname: string) {
  const index = dashboardNavigation.findIndex((item) => item.href === pathname);
  return index < 0 ? 0 : index;
}
