create table session
(
    id         uuid        not null default gen_random_uuid(),
    team       varchar(32) not null,
    created_at timestamp   not null,
    constraint session_pk primary key (id)
);

create index session_created_at_ix on session (created_at);
