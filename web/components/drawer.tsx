"use client";

import { PropsWithChildren, ReactNode, useEffect } from "react";

export function Drawer({
  open,
  onClose,
  children,
  actions
}: PropsWithChildren<{
  open: boolean;
  onClose: () => void;
  actions?: ReactNode;
}>) {
  useEffect(() => {
    if (!open) {
      return;
    }

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onClose, open]);

  if (!open) {
    return null;
  }

  return (
    <div
      aria-modal="true"
      className="drawer-backdrop"
      onClick={onClose}
      role="dialog"
    >
      <aside
        className="drawer-panel"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="drawer-head">
          <button
            aria-label="关闭"
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
    </div>
  );
}
