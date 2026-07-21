"use client";

import { FormEvent, useEffect, useRef, useState } from "react";

export function ChatHistorySearchForm({
  query,
  onQueryChange,
  ariaLabel = "搜索此群聊天记录",
  placeholder = "搜索消息、发言人或 @username",
}: {
  query: string;
  onQueryChange: (query: string) => void;
  ariaLabel?: string;
  placeholder?: string;
}) {
  const [draft, setDraft] = useState(query);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => setDraft(query), [query]);
  useEffect(() => inputRef.current?.focus(), []);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onQueryChange(draft.trim());
  }

  return (
    <form className="chat-history-search" onSubmit={submit} role="search">
      <SearchIcon />
      <input
        aria-label={ariaLabel}
        onChange={(event) => setDraft(event.target.value)}
        placeholder={placeholder}
        ref={inputRef}
        type="search"
        value={draft}
      />
      {draft ? (
        <button
          aria-label="清空聊天记录搜索"
          className="chat-history-search-clear"
          onClick={() => {
            setDraft("");
            onQueryChange("");
          }}
          type="button"
        >
          <CloseIcon />
        </button>
      ) : null}
      <button className="chat-history-search-submit" type="submit">搜索</button>
    </form>
  );
}

function SearchIcon() {
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24"><circle cx="10.8" cy="10.8" r="6.3" stroke="currentColor" strokeWidth="1.8" /><path d="m16 16 4 4" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" /></svg>;
}

function CloseIcon() {
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24"><path d="m7 7 10 10M17 7 7 17" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" /></svg>;
}
