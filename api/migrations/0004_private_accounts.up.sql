alter table users
    add column session_version integer not null default 0;

create unique index users_email_case_insensitive_idx on users (lower(email));
