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
              <span aria-hidden="true" className="nav-link-icon">
                <NavigationIcon name={item.icon} />
              </span>
              <span className="nav-link-label">{item.label}</span>
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
  { href: "/dashboard/chats", label: "群组", icon: "chats" },
  { href: "/dashboard/summaries", label: "摘要", icon: "summaries" },
  { href: "/dashboard/settings", label: "系统配置", icon: "settings" },
] as const;

function activeNavigationIndex(pathname: string) {
  const index = dashboardNavigation.findIndex((item) => item.href === pathname);
  return index < 0 ? 0 : index;
}

function NavigationIcon({ name }: { name: (typeof dashboardNavigation)[number]["icon"] }) {
  if (name === "chats") {
    return (
      <svg fill="none" viewBox="0 0 24 24">
        <path d="M7 18.5 3.5 21v-14A3.5 3.5 0 0 1 7 3.5h10A3.5 3.5 0 0 1 20.5 7v6A3.5 3.5 0 0 1 17 16.5H9.5L7 18.5Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
        <path d="M8 9.5h8M8 12.5h5" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      </svg>
    );
  }

  if (name === "summaries") {
    return (
      <svg fill="none" viewBox="0 0 24 24">
        <path d="M7 3.5h8.5L20 8v12.5H7a3 3 0 0 1-3-3v-11a3 3 0 0 1 3-3Z" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
        <path d="M15 3.5V8h5M8 12h8M8 15.5h6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      </svg>
    );
  }

  return (
    <svg fill="none" viewBox="0 0 24 24">
      <path d="M5 7h14M5 12h14M5 17h14" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M9 5.5v3M15 10.5v3M11 15.5v3" stroke="currentColor" strokeLinecap="round" strokeWidth="2.4" />
    </svg>
  );
}
