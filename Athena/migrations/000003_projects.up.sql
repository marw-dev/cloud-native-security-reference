create table projects (
    `id` varchar(36) not null,
    `name` varchar(255) not null,
    `owner_user_id` varchar(36) not null,
    `force_2fa` tinyint(1) not null default 0,
    `created_at` timestamp not null default current_timestamp,
    `updated_at` timestamp not null default current_timestamp on update current_timestamp,
    `host` varchar(255),

    primary key (`id`),
    unique (`host`),

    constraint `fk_owner_id`
        foreign key (`owner_user_id`)
        references `users` (`id`)
        on delete cascade
        on update cascade
);