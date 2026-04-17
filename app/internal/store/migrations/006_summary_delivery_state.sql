alter table summaries
add column if not exists delivered_at timestamptz;

alter table summaries
add column if not exists delivery_error text not null default '';
