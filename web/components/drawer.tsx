"use client";

import { PropsWithChildren, ReactNode, useEffect, useState } from "react";
import { createPortal } from "react-dom";

const drawerTransitionDuration = 220;

export function Drawer({
  open,
  onClose,
  children,
  actions,
  active = true,
  layer = "default",
  panelClassName,
  title
}: PropsWithChildren<{
  open: boolean;
  onClose: () => void;
  actions?: ReactNode;
  active?: boolean;
  layer?: "default" | "top";
  panelClassName?: string;
  title?: ReactNode;
}>) {
  const [rendered, setRendered] = useState(open);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    let firstFrame = 0;
    let secondFrame = 0;
    let exitTimer = 0;

    if (open) {
      setRendered(true);
      firstFrame = window.requestAnimationFrame(() => {
        secondFrame = window.requestAnimationFrame(() => setVisible(true));
      });
    } else {
      setVisible(false);
      if (rendered) {
        exitTimer = window.setTimeout(
          () => setRendered(false),
          prefersReducedMotion() ? 0 : drawerTransitionDuration,
        );
      }
    }

    return () => {
      window.cancelAnimationFrame(firstFrame);
      window.cancelAnimationFrame(secondFrame);
      window.clearTimeout(exitTimer);
    };
  }, [open, rendered]);

  useEffect(() => {
    if (!rendered) {
      return;
    }

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    function onKeyDown(event: KeyboardEvent) {
      if (active && open && event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [active, onClose, open, rendered]);

  if (!rendered) {
    return null;
  }

  const visibilityClass = visible ? " drawer-backdrop-visible" : " drawer-backdrop-hidden";

  return createPortal(
    <div
      aria-hidden={active && open ? undefined : true}
      aria-modal={active && open ? true : undefined}
      className={`drawer-backdrop${layer === "top" ? " drawer-backdrop-top" : ""}${visibilityClass}`}
      onClick={active && open ? onClose : undefined}
      role="dialog"
    >
      <aside
        className={`drawer-panel${panelClassName ? ` ${panelClassName}` : ""}`}
        onClick={(event) => event.stopPropagation()}
      >
        <div className={`drawer-head${title ? " drawer-head-with-title" : ""}`}>
          {title ? <h2>{title}</h2> : null}
          <button
            aria-label="关闭"
            autoFocus={layer === "top"}
            className="drawer-close"
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </div>
        <div className="drawer-body">{children}</div>
        {actions ? <div className="drawer-actions">{actions}</div> : null}
      </aside>
    </div>,
    document.body,
  );
}

function prefersReducedMotion() {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
