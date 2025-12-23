create table users (
    `id` char(36) not null,
    `email` varchar(255) not null,
    `password_hash` varchar(255) not null,
    `created_at` timestamp not null default current_timestamp,
    `updated_at` timestamp not null default current_timestamp on update current_timestamp,
    `is_admin` tinyint(1) not null default 0,
    `global_otp_enabled` tinyint(1) not null default 0,
    `global_otp_secret` varchar(255),
    `global_otp_auth_url` text,

    primary key (`id`),
    unique (`email`)
);