"use client";

import { PropsWithChildren } from "react";
import { DashboardProvider, useDashboard } from "@/lib/dashboard-context";
import { NavLink, StatusPill } from "@/components/ui";

function DashboardSidebar() {
	const { bootstrap } = useDashboard();
	return (
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
	);
}

export function DashboardShell({ children }: PropsWithChildren) {
	return (
		<DashboardProvider>
			<div className="dashboard-layout">
				<DashboardSidebar />
				<div className="dashboard-main">{children}</div>
			</div>
		</DashboardProvider>
	);
}
