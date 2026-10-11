alter table chats add column if not exists summary_frequency text not null default 'day';
alter table chats add column if not exists previous_summary_frequency text not null default 'day';
alter table chats add column if not exists summary_frequency_effective_date date;
alter table summaries add column if not exists period text not null default 'day';
alter table catch_up_sources add column if not exists period_end date;
