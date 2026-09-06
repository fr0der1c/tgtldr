alter table app_settings alter column summary_retry_limit set default 4;

-- 启用自动重试的实例统一采用四次阶梯重试，保留显式关闭的设置。
update app_settings set summary_retry_limit = 4 where summary_retry_limit > 0;
