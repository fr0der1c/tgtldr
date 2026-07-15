create extension if not exists pg_trgm;

create index if not exists idx_messages_search_trgm
on messages
using gin ((text_content || ' ' || caption || ' ' || sender_name || ' ' || sender_username) gin_trgm_ops);
