alter table app_settings
add column if not exists openai_output_mode text not null default 'auto';

alter table app_settings
add column if not exists summary_parallelism integer not null default 2;

update app_settings
set openai_output_mode = 'auto'
where openai_output_mode not in ('auto', 'manual');

update app_settings
set summary_parallelism = 2
where summary_parallelism < 1 or summary_parallelism > 6;
