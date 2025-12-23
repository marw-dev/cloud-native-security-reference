create table user_projects (
    `user_id` varchar(36) not null,
    `project_id` varchar(36) not null,
    `roles` text not null,
    `otp_enabled` tinyint(1) not null default 0,
    `otp_secret` varchar(255),
    `otp_auth_url` text,
    `created_at` timestamp not null default current_timestamp,
    `updated_at` timestamp not null default current_timestamp on update current_timestamp,

    primary key (`user_id`, `project_id`),

    constraint `fk_user_projects_user`
        foreign key (`user_id`)
        references `users` (`id`)
        on delete cascade
        on update cascade,
    
    constraint `fk_user_projects_project`
        foreign key (`project_id`)
        references `projects` (`id`)
        on delete cascade
        on update cascade
);