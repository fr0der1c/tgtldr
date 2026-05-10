alter table summaries
add column if not exists error_system_prompt text not null default '';

alter table summaries
add column if not exists error_user_prompt text not null default '';

alter table app_settings
add column if not exists summary_retry_limit integer not null default 2;

alter table app_settings
add column if not exists summary_retry_backoff_base_minutes integer not null default 1;

alter table app_settings
add column if not exists summary_retry_backoff_multiplier double precision not null default 3;

update app_settings
set summary_retry_limit = 2
where summary_retry_limit < 0;

update app_settings
set summary_retry_backoff_base_minutes = 1
where summary_retry_backoff_base_minutes < 1;

update app_settings
set summary_retry_backoff_multiplier = 3
where summary_retry_backoff_multiplier < 1;

alter table summaries
add column if not exists retry_count integer not null default 0;

alter table summaries
add column if not exists next_retry_at timestamptz;

update summaries
set retry_count = 0
where retry_count < 0;
