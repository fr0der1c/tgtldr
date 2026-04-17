alter table app_settings
add column if not exists default_timezone text not null default 'Asia/Shanghai';

alter table chats
add column if not exists summary_enabled boolean not null default false;

alter table chats
add column if not exists model_override text not null default '';

update chats
set summary_enabled = enabled
where summary_enabled = false and enabled = true;
