create table summary_recovery (
    kind text not null check (kind in ('summary', 'digest')),
    item_id bigint not null,
    summary_date date not null,
    started boolean not null default false,
    status text not null default 'queued',
    error_message text not null default '',
    primary key (kind, item_id)
);

create table summary_failure_notices (
    summary_date date primary key,
    message text not null,
    attempts integer not null default 0,
    next_attempt_at timestamptz not null default now(),
    delivered_at timestamptz,
    delivery_error text not null default ''
);
