create table if not exists catch_ups (
    id bigint primary key generated always as identity,
    from_date date not null,
    to_date date not null,
    status text not null default 'pending',
    content text not null default '',
    model text not null default '',
    chat_count integer not null default 0,
    source_summary_count integer not null default 0,
    chunk_count integer not null default 0,
    execution_mode text not null default '',
    estimated_input_tokens integer not null default 0,
    context_window_tokens integer not null default 0,
    fallback_reason text not null default '',
    delivery_requested boolean not null default false,
    delivered_at timestamptz,
    delivery_error text not null default '',
    error_message text not null default '',
    generated_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_catch_ups_created_at on catch_ups (created_at desc);

create table if not exists catch_up_chats (
    catch_up_id bigint not null references catch_ups(id) on delete cascade,
    chat_id bigint not null,
    chat_title text not null,
    source_summary_count integer not null default 0,
    primary key (catch_up_id, chat_id)
);

create table if not exists catch_up_sources (
    catch_up_id bigint not null references catch_ups(id) on delete cascade,
    summary_id bigint not null,
    chat_id bigint not null,
    chat_title text not null,
    summary_date date not null,
    content text not null,
    primary key (catch_up_id, summary_id)
);

create index if not exists idx_catch_up_sources_order
on catch_up_sources (catch_up_id, summary_date, chat_title, summary_id);
