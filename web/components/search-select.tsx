"use client";

import {
  ChangeEvent,
  KeyboardEvent as ReactKeyboardEvent,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";
import { Input } from "@/components/ui";

type SearchSelectOption = {
  value: string;
  label: string;
};

export function SearchSelect({
  emptyText = "没有匹配项",
  onChange,
  options,
  placeholder,
  searchPlaceholder,
  value
}: {
  emptyText?: string;
  onChange: (value: string) => void;
  options: SearchSelectOption[];
  placeholder: string;
  searchPlaceholder: string;
  value: string;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const selected = useMemo(
    () => options.find((option) => option.value === value) ?? null,
    [options, value]
  );

  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) {
      return options;
    }

    return options.filter((option) => option.label.toLowerCase().includes(normalized));
  }, [options, query]);

  useEffect(() => {
    if (!open) {
      return;
    }

    const timer = window.setTimeout(() => inputRef.current?.focus(), 20);
    return () => window.clearTimeout(timer);
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }

    function onPointerDown(event: MouseEvent) {
      if (rootRef.current?.contains(event.target as Node)) {
        return;
      }
      setOpen(false);
      setQuery("");
    }

    function onEscape(event: globalThis.KeyboardEvent) {
      if (event.key !== "Escape") {
        return;
      }
      setOpen(false);
      setQuery("");
    }

    window.addEventListener("mousedown", onPointerDown);
    window.addEventListener("keydown", onEscape);
    return () => {
      window.removeEventListener("mousedown", onPointerDown);
      window.removeEventListener("keydown", onEscape);
    };
  }, [open]);

  function openList() {
    setOpen(true);
  }

  function toggleList() {
    setOpen((current) => !current);
    setQuery("");
  }

  function closeList() {
    setOpen(false);
    setQuery("");
  }

  function selectValue(nextValue: string) {
    onChange(nextValue);
    closeList();
  }

  function onButtonKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>) {
    if (event.key !== "ArrowDown" && event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    openList();
  }

  function onInputChange(event: ChangeEvent<HTMLInputElement>) {
    setQuery(event.target.value);
  }

  return (
    <div className={`search-select ${open ? "open" : ""}`} ref={rootRef}>
      <button
        aria-expanded={open}
        className="search-select-trigger"
        onClick={toggleList}
        onKeyDown={onButtonKeyDown}
        type="button"
      >
        <span>{selected?.label ?? placeholder}</span>
        <span aria-hidden="true" className="search-select-chevron">
          ▾
        </span>
      </button>

      {open ? (
        <div className="search-select-panel" role="listbox">
          <Input
            onChange={onInputChange}
            placeholder={searchPlaceholder}
            ref={inputRef}
            value={query}
          />
          <div className="search-select-options">
            {filtered.length === 0 ? (
              <div className="search-select-empty">{emptyText}</div>
            ) : (
              filtered.map((option) => (
                <button
                  aria-selected={option.value === value}
                  className={`search-select-option ${
                    option.value === value ? "selected" : ""
                  }`}
                  key={option.value}
                  onClick={() => selectValue(option.value)}
                  type="button"
                >
                  <span>{option.label}</span>
                  {option.value === value ? (
                    <span aria-hidden="true" className="search-select-check">
                      ✓
                    </span>
                  ) : null}
                </button>
              ))
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}
