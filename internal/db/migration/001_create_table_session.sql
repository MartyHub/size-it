create table session
(
    id         varchar(26) not null,
    team       varchar(32) not null,
    created_at timestamp   not null,
    constraint session_pk primary key (id)
);

create index session_created_at_ix on session (created_at);
