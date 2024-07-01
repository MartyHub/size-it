-- name: Session :one
select *
  from session
 where id = @id
;

-- name: CreateSession :one
insert into session
    (team, created_at) values
    (@team, @created_at)
returning *
;

-- name: Teams :many
select distinct team
  from session
 where created_at >= 'now'::timestamp - '3 month'::interval
 order by team
;

-- name: CreateTicket :one
insert into ticket
    (session_id, summary, url, sizing_type, sizing_value) values
    (@session_id, @summary, @url, @sizing_type, @sizing_value)
returning *
;

-- name: UpdateTicket :exec
update ticket set
    summary      = @summary,
    url          = @url,
    sizing_type  = @sizing_type,
    sizing_value = @sizing_value
where id = @id
;

-- name: History :many
select t.*
  from ticket t
 inner join session s on s.id = t.session_id
                     and s.team = @team
                     and s.created_at >= 'now'::timestamp - '3 month'::interval
 where t.sizing_type = @sizing_type
 order by t.id desc
;
