alter table summaries add column if not exists requested_model text not null default '';
alter table summaries add column if not exists returned_model text not null default '';
