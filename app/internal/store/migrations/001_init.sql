create table if not exists app_settings (
    id bigint primary key generated always as identity,
    telegram_api_id integer not null default 0,
    telegram_api_hash text not null default '',
    openai_base_url text not null default '',
    openai_api_key text not null default '',
    openai_model text not null default 'gpt-4.1-mini',
    openai_temperature double precision not null default 0.2,
    openai_max_output_tokens integer not null default 2000,
    bot_enabled boolean not null default false,
    bot_token text not null default '',
    bot_target_chat_id text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

insert into app_settings (telegram_api_id)
select 0
where not exists (select 1 from app_settings);

create table if not exists telegram_auth (
    id bigint primary key generated always as identity,
    phone_number text not null default '',
    telegram_user_id bigint not null default 0,
    telegram_name text not null default '',
    telegram_handle text not null default '',
    session_data text not null default '',
    status text not null default 'logged_out',
    last_connected_at timestamptz not null default now(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists chats (
    id bigint primary key generated always as identity,
    telegram_chat_id bigint not null unique,
    telegram_access_hash bigint not null default 0,
    title text not null,
    username text not null default '',
    chat_type text not null default 'group',
    enabled boolean not null default false,
    summary_prompt text not null default '',
    summary_time_local text not null default '09:00',
    summary_timezone text not null default 'Asia/Shanghai',
    delivery_mode text not null default 'dashboard',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table if not exists messages (
    id bigint primary key generated always as identity,
    chat_id bigint not null references chats(id) on delete cascade,
    telegram_message_id integer not null,
    telegram_sender_id bigint not null default 0,
    sender_name text not null default '',
    text_content text not null default '',
    caption text not null default '',
    message_type text not null default 'text',
    media_kind text not null default '',
    reply_to_message_id integer not null default 0,
    message_time timestamptz not null,
    raw_json jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    unique (chat_id, telegram_message_id)
);

create index if not exists idx_messages_chat_time on messages (chat_id, message_time);

create table if not exists summaries (
    id bigint primary key generated always as identity,
    chat_id bigint not null references chats(id) on delete cascade,
    summary_date date not null,
    status text not null default 'pending',
    content text not null default '',
    model text not null default '',
    source_message_count integer not null default 0,
    chunk_count integer not null default 0,
    generated_at timestamptz not null default now(),
    error_message text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (chat_id, summary_date)
);

create index if not exists idx_summaries_chat_date on summaries (chat_id, summary_date desc);
