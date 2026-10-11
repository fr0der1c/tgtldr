alter table app_settings
    add column if not exists default_chat_delivery_mode text not null default 'dashboard',
    add column if not exists default_chat_summary_time_local text not null default '09:00',
    add column if not exists default_chat_model_override text not null default '',
    add column if not exists default_chat_keep_bot_messages boolean not null default true;
