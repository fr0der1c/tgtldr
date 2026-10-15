alter table telegram_accounts
    add column if not exists chats_synced_at timestamptz;
