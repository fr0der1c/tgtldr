alter table summaries
    add column if not exists delivery_retry_count integer not null default 0,
    add column if not exists next_delivery_retry_at timestamptz;
