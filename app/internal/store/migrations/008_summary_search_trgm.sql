create extension if not exists pg_trgm;

create index if not exists idx_summaries_content_trgm
on summaries
using gin (content gin_trgm_ops);

create index if not exists idx_chats_title_trgm
on chats
using gin (title gin_trgm_ops);
