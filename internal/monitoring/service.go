package monitoring

import (
	"context"
	"time"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/db"
)

type service struct {
	clk       internal.Clock
	repo      *db.Repository
	startTime time.Time
}

func newService(clk internal.Clock, repo *db.Repository) *service {
	return &service{
		clk:       clk,
		repo:      repo,
		startTime: clk.Now(),
	}
}

func (svc *service) health(ctx context.Context) HealthOutput {
	res := HealthOutput{
		Status: "UP",
		Uptime: svc.clk.Now().Sub(svc.startTime).String(),
	}

	if err := svc.repo.Ping(ctx); err != nil {
		res.Status = err.Error()
	}

	return res
}

func (svc *service) info(ctx context.Context) (InfoOutput, error) {
	dbInfo, err := svc.dbInfo(ctx)
	if err != nil {
		return InfoOutput{}, err
	}

	res := InfoOutput{
		Application: newApplicationInfo(),
		Database:    dbInfo,
		Go:          newGoInfo(),
	}

	return res, nil
}

func (svc *service) dbInfo(ctx context.Context) (DatabaseInfoOutput, error) {
	timezone, err := svc.repo.Timezone(ctx)
	if err != nil {
		return DatabaseInfoOutput{}, err
	}

	version, err := svc.repo.Version(ctx)
	if err != nil {
		return DatabaseInfoOutput{}, err
	}

	res := DatabaseInfoOutput{
		Timezone: timezone,
		Version:  version,
	}

	return res, nil
}
