alter table chats
add column if not exists alert_enabled boolean not null default false;

alter table chats
add column if not exists alert_keywords text[] not null default '{}';
