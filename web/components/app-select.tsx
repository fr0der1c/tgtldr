"use client";

import { KeyboardEvent as ReactKeyboardEvent, useEffect, useMemo, useRef, useState } from "react";

export type AppSelectOption = {
  label: string;
  value: string;
};

export function AppSelect({
  onChange,
  options,
  value
}: {
  onChange: (value: string) => void;
  options: AppSelectOption[];
  value: string;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const selected = useMemo(
    () => options.find((option) => option.value === value) ?? options[0] ?? null,
    [options, value]
  );

  useEffect(() => {
    if (!open) {
      return;
    }

    function onPointerDown(event: MouseEvent) {
      if (rootRef.current?.contains(event.target as Node)) {
        return;
      }
      setOpen(false);
    }

    function onEscape(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }

    window.addEventListener("mousedown", onPointerDown);
    window.addEventListener("keydown", onEscape);
    return () => {
      window.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("keydown", onEscape);
    };
  }, [open]);

  function onTriggerKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>) {
    if (event.key !== "ArrowDown" && event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    setOpen(true);
  }

  return (
    <div className={`app-select ${open ? "open" : ""}`} ref={rootRef}>
      <button
        aria-expanded={open}
        className="app-select-trigger"
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onTriggerKeyDown}
        type="button"
      >
        <span>{selected?.label ?? ""}</span>
        <span aria-hidden="true" className="app-select-chevron">
          ▾
        </span>
      </button>

      {open ? (
        <div className="app-select-panel" role="listbox">
          {options.map((option) => (
            <button
              aria-selected={option.value === value}
              className={`app-select-option ${option.value === value ? "selected" : ""}`}
              key={option.value}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
              type="button"
            >
              <span>{option.label}</span>
              {option.value === value ? (
                <span aria-hidden="true" className="app-select-check">
                  ✓
                </span>
              ) : null}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
