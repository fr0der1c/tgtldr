create table if not exists local_auth (
    id bigint primary key generated always as identity,
    password_hash text not null default '',
    password_updated_at timestamptz not null default now(),
    session_version integer not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

insert into local_auth (password_hash)
select ''
where not exists (select 1 from local_auth);

create table if not exists local_sessions (
    id bigint primary key generated always as identity,
    session_id text not null unique,
    session_version integer not null default 1,
    expires_at timestamptz not null,
    last_seen_at timestamptz not null default now(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists idx_local_sessions_expires_at on local_sessions (expires_at);
