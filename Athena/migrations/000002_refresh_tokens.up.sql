create table refresh_tokens (
    `token_hash` varchar(64) not null,
    `user_id` char(36) not null,
    `expires_at` timestamp not null,
    `created_at` timestamp not null default current_timestamp,
    `revoked` tinyint(1) not null default 0,

    primary key (`token_hash`),

    constraint `fk_user_id`
        foreign key (`user_id`)
        references `users` (`id`)
        on delete cascade
        on update cascade
);