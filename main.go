package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/db"
	"github.com/MartyHub/size-it/internal/monitoring"
	"github.com/MartyHub/size-it/internal/server"
	"github.com/MartyHub/size-it/internal/session"
)

func main() {
	internal.ConfigureLogs(false)

	if err := run(); err != nil {
		internal.LogError("Fatal error", err)

		os.Exit(1)
	}

	slog.Info("Bye")
}

func run() error {
	cfg, err := internal.ParseConfig()
	if err != nil {
		return err
	}

	internal.ConfigureLogs(cfg.Dev)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := db.NewRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}

	defer repo.Close()

	if err = repo.Migrate(ctx); err != nil {
		return err
	}

	srv := server.NewServer(cfg, repo)

	monitoring.Register(srv)
	session.Register(srv)

	return srv.Run(ctx)
}
