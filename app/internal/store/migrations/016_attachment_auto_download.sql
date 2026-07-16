alter table app_settings
    add column if not exists auto_download_attachments boolean not null default true;
