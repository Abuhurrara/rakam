-- Keep receipts independently of transactions: retrying a deleted entry must
-- acknowledge the original save, never recreate it.
create table transaction_requests (
    user_id uuid not null references users(id) on delete cascade,
    request_key text not null,
    request_hash text not null,
    response jsonb,
    created_at timestamptz not null default now(),
    primary key (user_id, request_key)
);
