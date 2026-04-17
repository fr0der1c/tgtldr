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

export function MetricRail({ children }: PropsWithChildren) {
  return <div className="metric-rail">{children}</div>;
}

export function MetricCard({
  label,
  value,
  tone = "neutral",
  detail,
  badge
}: {
  label: string;
  value: string | number;
  tone?: "neutral" | "good" | "warn" | "bad";
  detail?: string;
  badge?: string;
}) {
  return (
    <article className="metric-card">
      <div className="metric-card-head">
        <span>{label}</span>
        {badge ? <StatusPill tone={tone}>{badge}</StatusPill> : null}
      </div>
      <strong>{value}</strong>
      {detail ? <p>{detail}</p> : null}
    </article>
  );
}

export function Surface({
  title,
  description,
  actions,
  children
}: PropsWithChildren<{
  title?: string;
  description?: string;
  actions?: ReactNode;
}>) {
  return (
    <section className="dashboard-surface">
      {title || description || actions ? (
        <div className="dashboard-surface-head">
          <div>
            {title ? <h2>{title}</h2> : null}
            {description ? <p>{description}</p> : null}
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
