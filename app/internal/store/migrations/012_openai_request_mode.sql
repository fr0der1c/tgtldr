alter table app_settings
add column if not exists openai_request_mode text not null default 'stream';

update app_settings
set openai_request_mode = 'stream'
where openai_request_mode not in ('stream', 'non_stream');
