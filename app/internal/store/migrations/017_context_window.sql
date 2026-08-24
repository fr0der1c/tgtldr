alter table app_settings
add column if not exists openai_context_window_mode text not null default 'auto';

alter table app_settings
add column if not exists openai_context_window_tokens integer not null default 0;

update app_settings
set openai_context_window_mode = 'auto'
where openai_context_window_mode not in ('auto', 'manual');
