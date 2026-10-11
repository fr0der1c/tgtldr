create table if not exists summary_batch_items (
    chat_id bigint not null references chats(id) on delete cascade,
    summary_date date not null,
    position integer not null,
    overwrite boolean not null default false,
    started_at timestamptz,
    status text not null default 'queued' check (status in ('queued', 'succeeded', 'failed', 'skipped')),
    error_message text not null default '',
    primary key (chat_id, summary_date)
);
