"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ButtonHTMLAttributes,
  forwardRef,
  InputHTMLAttributes,
  PropsWithChildren,
  ReactNode,
  SelectHTMLAttributes,
  TextareaHTMLAttributes
} from "react";

export function PageShell({
  title,
  eyebrow,
  actions,
  children
}: PropsWithChildren<{ title: string; eyebrow?: string; actions?: ReactNode }>) {
  return (
    <main className="page-shell">
      <section className="hero-card">
        {eyebrow ? <p className="eyebrow">{eyebrow}</p> : null}
        <div className="hero-row">
          <div>
            <h1>{title}</h1>
          </div>
          {actions ? <div className="hero-actions">{actions}</div> : null}
        </div>
      </section>
      <section className="content-grid">{children}</section>
    </main>
  );
}

export function Card({
  title,
  description,
  children
}: PropsWithChildren<{ title?: string; description?: string }>) {
  return (
    <article className="card">
      {title ? <h2>{title}</h2> : null}
      {description ? <p className="muted">{description}</p> : null}
      {children}
    </article>
  );
}

export function Field({
  as = "label",
  label,
  hint,
  required,
  children
}: PropsWithChildren<{
  as?: "label" | "div";
  label: string;
  hint?: string;
  required?: boolean;
}>) {
  const Wrapper = as;
  return (
    <Wrapper className="field">
      <span className="field-label">
        {label}
        {required ? <span className="field-required">必填</span> : null}
      </span>
      {hint ? <span className="field-hint">{hint}</span> : null}
      {children}
    </Wrapper>
  );
}

export function Button({
  children,
  variant = "primary",
  className,
  ...props
}: PropsWithChildren<
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: "primary" | "secondary" | "ghost" | "link";
  }
>) {
  return (
    <button className={`button ${variant}${className ? ` ${className}` : ""}`} {...props}>
      {children}
    </button>
  );
}

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input(props, ref) {
    return <input className="input" ref={ref} {...props} />;
  }
);

export function Textarea(
  props: TextareaHTMLAttributes<HTMLTextAreaElement>
) {
  return <textarea className="textarea" {...props} />;
}

export function Select(
  props: SelectHTMLAttributes<HTMLSelectElement>
) {
  return <select className="input" {...props} />;
}

export function StatusPill({
  tone,
  title,
  className,
  children
}: PropsWithChildren<{
  tone: "neutral" | "good" | "warn" | "bad";
  title?: string;
  className?: string;
}>) {
  return (
    <span className={`status-pill ${tone}${className ? ` ${className}` : ""}`} title={title}>
      {children}
    </span>
  );
}

export function NavLink({
  href,
  children
}: PropsWithChildren<{ href: string }>) {
  const pathname = usePathname();
  const active = pathname === href;

  return (
    <Link className={`nav-link ${active ? "active" : ""}`} href={href}>
      {children}
    </Link>
  );
}
