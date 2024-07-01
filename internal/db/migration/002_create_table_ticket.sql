create table ticket
(
    id           bigint       not null generated always as identity,
    session_id   uuid         not null,
    summary      varchar(512) not null,
    url          varchar(512) not null,
    sizing_type  varchar(16)  not null,
    sizing_value varchar(8)   not null,
    constraint ticket_pk primary key (id)
);

alter table ticket
    add constraint ticket_session_id foreign key (session_id) references session (id);

create index ticket_session_ix on ticket (session_id);
