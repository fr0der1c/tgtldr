alter table chats
add column if not exists rolling_summary_enabled boolean not null default false;

alter table chats
add column if not exists rolling_summary_interval_minutes integer not null default 180;

alter table chats
add column if not exists rolling_summary_max_per_day integer not null default 5;

alter table chats
add column if not exists rolling_summary_bot_enabled boolean not null default true;

update chats
set rolling_summary_interval_minutes = 180
where rolling_summary_interval_minutes < 1;

update chats
set rolling_summary_max_per_day = 5
where rolling_summary_max_per_day < 1;

alter table summaries
add column if not exists summary_type text not null default 'daily';

alter table summaries
add column if not exists window_start timestamptz;

alter table summaries
add column if not exists window_end timestamptz;

update summaries
set summary_type = 'daily'
where summary_type = '';

alter table summaries
drop constraint if exists summaries_chat_id_summary_date_key;

create unique index if not exists idx_summaries_daily_unique
on summaries (chat_id, summary_date, summary_type)
where summary_type = 'daily';

create unique index if not exists idx_summaries_rolling_unique
on summaries (chat_id, summary_type, window_end)
where summary_type = 'rolling';

create index if not exists idx_summaries_chat_type_date
on summaries (chat_id, summary_type, summary_date desc, window_end desc);
