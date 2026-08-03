create index if not exists idx_messages_chat_sender_time
on messages (chat_id, telegram_sender_id, message_time, telegram_message_id);

create index if not exists idx_messages_chat_time_message
on messages (chat_id, message_time, telegram_message_id);

create index if not exists idx_messages_chat_sender_username
on messages (chat_id, lower(sender_username), telegram_sender_id)
where sender_username <> '';
