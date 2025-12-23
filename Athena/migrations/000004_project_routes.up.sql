CREATE TABLE project_routes (
    `id` varchar(36) not null,
    `project_id` varchar(36) not null,
    `path` varchar(256) not null,
    `target_url` varchar(256) not null,
    `required_roles` text null default null,
    `rate_limit_limit` int default 0,
    `rate_limit_window` varchar(50) default '0s',
    `cache_ttl` varchar(50) default '0s',
    `cb_threshold` int default 0,
    `cb_timeout` varchar(50) default '0s',
    `created_at` timestamp not null default current_timestamp,
    `updated_at` timestamp not null default current_timestamp on update current_timestamp,

    primary key (`id`),
    
    constraint `fk_project_id`
        foreign key (`project_id`)
        references `projects` (`id`)
        on delete cascade
        on update cascade
);