alter table app_settings
add column if not exists bot_summary_delivery_mode text not null default 'per_chat';

alter table app_settings
add column if not exists previous_bot_summary_delivery_mode text not null default 'per_chat';

alter table app_settings
add column if not exists bot_summary_delivery_mode_effective_date date not null default '1970-01-01';

alter table summaries
add column if not exists bot_summary_delivery_mode text not null default 'per_chat';

create table if not exists daily_digests (
    id bigint primary key generated always as identity,
    summary_date date not null unique,
    status text not null default 'pending',
    content text not null default '',
    model text not null default '',
    participant_count integer not null default 0,
    source_summary_count integer not null default 0,
    empty_chat_count integer not null default 0,
    omitted_chat_count integer not null default 0,
    chunk_count integer not null default 0,
    execution_mode text not null default '',
    estimated_input_tokens integer not null default 0,
    context_window_tokens integer not null default 0,
    fallback_reason text not null default '',
    delivery_skipped_reason text not null default '',
    delivery_suppressed boolean not null default false,
    delivered_at timestamptz,
    delivery_error text not null default '',
    error_message text not null default '',
    retry_count integer not null default 0,
    next_retry_at timestamptz,
    generated_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_daily_digests_created_at
on daily_digests (created_at desc, id desc);

create table if not exists daily_digest_sources (
    daily_digest_id bigint not null references daily_digests(id) on delete cascade,
    summary_id bigint not null default 0,
    chat_id bigint not null,
    chat_title text not null,
    summary_status text not null,
    source_message_count integer not null default 0,
    included boolean not null default false,
    omission_reason text not null default '',
    content text not null default '',
    model text not null default '',
    primary key (daily_digest_id, chat_id)
);

create index if not exists idx_daily_digest_sources_order
on daily_digest_sources (daily_digest_id, included desc, chat_title, chat_id);

create unique index if not exists idx_daily_digest_sources_summary
on daily_digest_sources (summary_id)
where summary_id > 0;
