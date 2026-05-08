-- need 1: keyword alert per chat
alter table chats
    add column if not exists alert_enabled boolean not null default false,
    add column if not exists alert_keywords text[] not null default '{}';

-- need 7: global default settings
alter table app_settings
    add column if not exists default_delivery_mode text not null default 'dashboard',
    add column if not exists default_summary_time_local text not null default '09:00',
    add column if not exists default_model_override text not null default '',
    add column if not exists default_keep_bot_messages boolean not null default true;

-- need 9: bot delivery auto-retry tracking
alter table summaries
    add column if not exists delivery_retry_count integer not null default 0,
    add column if not exists next_delivery_retry_at timestamptz;

-- need 10: summary frequency per chat (daily / weekly / monthly)
alter table chats
    add column if not exists summary_frequency text not null default 'daily';
