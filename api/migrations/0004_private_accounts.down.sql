drop index if exists users_email_case_insensitive_idx;
alter table users drop column if exists session_version;
