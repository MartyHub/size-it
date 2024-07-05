package session

import (
	"github.com/MartyHub/size-it/internal/db/sqlc"
)

func toSession(entity sqlc.Session) Session {
	return Session{
		ID:        entity.ID,
		Team:      entity.Team,
		CreatedAt: entity.CreatedAt.Time,
	}
}
