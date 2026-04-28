alter table app_settings
add column if not exists language text not null default 'zh-CN';

update app_settings
set language = 'zh-CN'
where language not in ('zh-CN', 'en');
