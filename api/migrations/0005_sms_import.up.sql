create table import_pairing_codes (
    code_hash text primary key,
    user_id uuid not null references users(id) on delete cascade,
    expires_at timestamptz not null,
    created_at timestamptz not null default now()
);

create table import_devices (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    token_hash text not null unique,
    name text not null,
    created_at timestamptz not null default now(),
    last_seen_at timestamptz,
    revoked_at timestamptz
);
create index import_devices_user_id_idx on import_devices (user_id);

create table import_suggestions (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    source text not null check (source = 'meezan_sms'),
    source_id text not null,
    kind text not null check (kind in ('expense', 'income')),
    amount_paisa bigint not null check (amount_paisa > 0),
    occurred_at timestamptz not null,
    description text not null,
    status text not null default 'pending' check (status in ('pending', 'confirmed', 'ignored')),
    transaction_id uuid references transactions(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, source, source_id)
);
create index import_suggestions_user_status_idx on import_suggestions (user_id, status, occurred_at desc);
