alter table chats
add column if not exists summary_context text not null default '';
