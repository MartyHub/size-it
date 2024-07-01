package session

import (
	"fmt"

	"github.com/MartyHub/size-it/internal/db/sqlc"
)

func toSession(entity sqlc.Session) Session {
	val, _ := entity.ID.Value()

	return Session{
		ID:        fmt.Sprint(val),
		Team:      entity.Team,
		CreatedAt: entity.CreatedAt.Time,
	}
}
