alter table telegram_auth rename to telegram_accounts;

alter table chats
    add column collector_account_id bigint references telegram_accounts(id) on delete restrict;

create table telegram_account_chats (
    telegram_account_id bigint not null references telegram_accounts(id) on delete cascade,
    chat_id bigint not null references chats(id) on delete cascade,
    telegram_access_hash bigint not null default 0,
    last_synced_at timestamptz not null default now(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (telegram_account_id, chat_id)
);

create index idx_telegram_account_chats_chat_id
    on telegram_account_chats (chat_id);

insert into telegram_account_chats (telegram_account_id, chat_id, telegram_access_hash)
select account.id, chat.id, chat.telegram_access_hash
from chats chat
cross join lateral (
    select id from telegram_accounts order by id desc limit 1
) account
on conflict do nothing;

update chats chat
set collector_account_id = account.id
from (
    select id from telegram_accounts order by id desc limit 1
) account
where chat.collector_account_id is null;
