alter table messages
    add column if not exists sender_entity_id bigint;

create table telegram_entities (
    id bigint primary key generated always as identity,
    telegram_account_id bigint not null references telegram_accounts(id) on delete cascade,
    peer_type text not null,
    telegram_id bigint not null,
    access_hash bigint not null default 0,
    display_name text not null default '',
    username text not null default '',
    current_photo_id bigint not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (telegram_account_id, peer_type, telegram_id)
);

alter table messages
    add constraint messages_sender_entity_id_fkey
    foreign key (sender_entity_id) references telegram_entities(id) on delete set null;

create table media_assets (
    id bigint primary key generated always as identity,
    telegram_account_id bigint references telegram_accounts(id) on delete set null,
    owner_type text not null,
    message_id bigint references messages(id) on delete cascade,
    entity_id bigint references telegram_entities(id) on delete cascade,
    photo_id bigint not null default 0,
    kind text not null,
    mime_type text not null default 'application/octet-stream',
    file_name text not null default '',
    file_size bigint not null default 0,
    location_type text not null,
    telegram_file_id bigint not null default 0,
    telegram_access_hash bigint not null default 0,
    file_reference bytea not null default ''::bytea,
    thumb_size text not null default '',
    peer_type text not null default '',
    peer_id bigint not null default 0,
    peer_access_hash bigint not null default 0,
    status text not null default 'pending',
    force_download boolean not null default false,
    local_path text not null default '',
    retry_count integer not null default 0,
    next_retry_at timestamptz,
    error_message text not null default '',
    downloaded_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check ((owner_type = 'message' and message_id is not null and entity_id is null) or
           (owner_type = 'avatar' and entity_id is not null and message_id is null))
);

create unique index media_assets_message_owner
    on media_assets (message_id) where owner_type = 'message';
create unique index media_assets_avatar_owner
    on media_assets (entity_id, photo_id) where owner_type = 'avatar';
create index media_assets_pending
    on media_assets (telegram_account_id, status, next_retry_at, created_at);
