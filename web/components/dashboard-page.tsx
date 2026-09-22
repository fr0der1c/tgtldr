"use client";

import Link from "next/link";
import { PropsWithChildren, ReactNode } from "react";
import { StatusPill } from "@/components/ui";

export function DashboardPage({
  title,
  description,
  actions,
  children
}: PropsWithChildren<{
  title: string;
  description?: string;
  actions?: ReactNode;
}>) {
  return (
    <section className="dashboard-page">
      <header className="dashboard-page-head">
        <div className="dashboard-page-copy">
          <p className="dashboard-page-kicker">TGTLDR</p>
          <h1>{title}</h1>
          {description ? <p>{description}</p> : null}
        </div>
        {actions ? <div className="dashboard-page-actions">{actions}</div> : null}
      </header>
      <div className="dashboard-page-body">{children}</div>
    </section>
  );
}

/** MetricRail 可按页面层级选择标准或紧凑统计布局。 */
export function MetricRail({
  children,
  compact = false,
}: PropsWithChildren<{ compact?: boolean }>) {
  return <div className={`metric-rail${compact ? " metric-rail-compact" : ""}`}>{children}</div>;
}

/** 统计卡可独立提供筛选入口与操作按钮，避免交互控件相互嵌套。 */
export function MetricCard({
  label,
  value,
  tone = "neutral",
  detail,
  badge,
  onClick,
  actions,
}: {
  label: string;
  value: string | number;
  tone?: "neutral" | "good" | "warn" | "bad";
  detail?: string;
  badge?: string;
  onClick?: () => void;
  actions?: ReactNode;
}) {
  const head = <div className="metric-card-head">
    <span>{label}</span>
    {badge ? <StatusPill tone={tone}>{badge}</StatusPill> : null}
  </div>;
  const content = (
    <>
      {head}
      <strong>{value}</strong>
      {detail ? <p>{detail}</p> : null}
    </>
  );

  if (actions) {
    return <article className="metric-card metric-card-with-actions">
      {onClick ? <button className="metric-card-select" type="button" aria-label={label} title={detail} onClick={onClick} /> : null}
      {head}
      <div className="metric-card-value-row">
        <strong>{value}</strong>
        <div className="metric-card-controls">{actions}</div>
      </div>
      {detail ? <p>{detail}</p> : null}
    </article>;
  }

  if (onClick) {
    return (
      <button className="metric-card metric-card-action" onClick={onClick} title={detail} type="button">
        {content}
      </button>
    );
  }

  return <article className="metric-card">{content}</article>;
}

export function Surface({
  title,
  description,
  actions,
  className,
  leading,
  children
}: PropsWithChildren<{
  title?: ReactNode;
  description?: string;
  actions?: ReactNode;
  className?: string;
  leading?: ReactNode;
}>) {
  return (
    <section className={`dashboard-surface${className ? ` ${className}` : ""}`}>
      {title || description || actions ? (
        <div className="dashboard-surface-head">
          <div className="dashboard-surface-heading">
            {leading ? <div className="dashboard-surface-leading">{leading}</div> : null}
            <div>
              {title ? <h2>{title}</h2> : null}
              {description ? <p>{description}</p> : null}
            </div>
          </div>
          {actions ? <div className="dashboard-surface-actions">{actions}</div> : null}
        </div>
      ) : null}
      <div className="dashboard-surface-body">{children}</div>
    </section>
  );
}

export function EmptyState({
  title,
  description,
  actionHref,
  actionLabel
}: {
  title: string;
  description: string;
  actionHref?: string;
  actionLabel?: string;
}) {
  return (
    <div className="empty-state">
      <h3>{title}</h3>
      <p>{description}</p>
      {actionHref && actionLabel ? (
        <Link className="empty-state-link" href={actionHref}>
          {actionLabel}
        </Link>
      ) : null}
    </div>
  );
}
