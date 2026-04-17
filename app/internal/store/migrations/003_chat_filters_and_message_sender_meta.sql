alter table chats
add column if not exists keep_bot_messages boolean not null default true;

alter table chats
add column if not exists filtered_senders text[] not null default '{}';

alter table chats
add column if not exists filtered_keywords text[] not null default '{}';

alter table messages
add column if not exists sender_username text not null default '';

alter table messages
add column if not exists sender_is_bot boolean not null default false;
