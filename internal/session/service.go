package session

import (
	"context"
	"fmt"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/db"
	"github.com/MartyHub/size-it/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type service struct {
	clk  internal.Clock
	repo *db.Repository
}

func newService(clk internal.Clock, repo *db.Repository) *service {
	return &service{
		clk:  clk,
		repo: repo,
	}
}

func (svc *service) create(ctx context.Context, input CreateOrJoinSessionInput) (Session, error) {
	id, err := db.NewID()
	if err != nil {
		return Session{}, err
	}

	entity, err := svc.repo.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        id,
		Team:      input.Team,
		CreatedAt: pgtype.Timestamp{Time: svc.clk.Now(), Valid: true},
	})
	if err != nil {
		return Session{}, err
	}

	return toSession(entity), nil
}

func (svc *service) get(ctx context.Context, id string) (Session, error) {
	entity, err := svc.repo.Session(ctx, id)
	if err != nil {
		if db.IsErrNoRows(err) {
			return Session{}, fmt.Errorf("%w: session %s", internal.ErrNotFound, id)
		}

		return Session{}, err
	}

	return toSession(entity), nil
}

func (svc *service) teams(ctx context.Context) ([]string, error) {
	teams, err := svc.repo.Teams(ctx)
	if err != nil {
		return nil, err
	}

	if teams == nil {
		teams = []string{}
	}

	return teams, nil
}
