create table users (
    id uuid primary key default gen_random_uuid(),
    email text not null unique,
    password_hash text not null,
    name text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table categories (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    name text not null,
    kind text not null check (kind in ('expense', 'income')),
    icon text not null,
    color text not null,
    sort_order int not null default 0,
    is_archived boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, name, kind)
);

create table people (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    name text not null,
    phone text,
    notes text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, name)
);

create table recurring_bills (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    name text not null,
    amount_paisa bigint not null,
    category_id uuid references categories(id),
    day_of_month int not null check (day_of_month between 1 and 31),
    is_active boolean not null default true,
    last_generated_month date,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table transactions (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    kind text not null check (kind in ('expense', 'income')),
    amount_paisa bigint not null,
    category_id uuid references categories(id),
    description text,
    occurred_at timestamptz not null,
    recurring_bill_id uuid references recurring_bills(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index transactions_user_id_occurred_at_idx on transactions (user_id, occurred_at desc);

create table debt_entries (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    person_id uuid not null references people(id),
    direction text not null check (direction in ('i_owe', 'they_owe')),
    amount_paisa bigint not null,
    description text not null,
    incurred_at timestamptz not null,
    settled_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table budgets (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    category_id uuid not null references categories(id),
    month date not null,
    limit_paisa bigint not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, category_id, month)
);

create table work_logs (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    month date not null,
    content text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (user_id, month)
);
