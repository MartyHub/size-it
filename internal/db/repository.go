package db

import (
	"context"
	"crypto/rand"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/MartyHub/size-it/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
	"github.com/oklog/ulid/v2"
)

//go:embed migration/*.sql
var migrations embed.FS

type Repository struct {
	*sqlc.Queries

	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, connString string) (*Repository, error) {
	slog.Info("Connecting to database...")

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	result := &Repository{
		Queries: sqlc.New(pool),
		pool:    pool,
	}

	return result, nil
}

func (repo *Repository) Close() {
	repo.pool.Close()
}

func (repo *Repository) Migrate(ctx context.Context) error {
	slog.Info("Migrating database...")

	return repo.pool.AcquireFunc(ctx, func(conn *pgxpool.Conn) error {
		return repo.doMigrate(ctx, conn.Conn())
	})
}

func (repo *Repository) Ping(ctx context.Context) error {
	return repo.pool.Ping(ctx)
}

func (repo *Repository) Timezone(ctx context.Context) (string, error) {
	rows, err := repo.pool.Query(ctx, "show timezone")
	if err != nil {
		return "", err
	}

	version, err := pgx.CollectOneRow[string](rows, pgx.RowTo[string])
	if err != nil {
		return "", err
	}

	return version, nil
}

func (repo *Repository) Version(ctx context.Context) (string, error) {
	rows, err := repo.pool.Query(ctx, "select version()")
	if err != nil {
		return "", err
	}

	version, err := pgx.CollectOneRow[string](rows, pgx.RowTo[string])
	if err != nil {
		return "", err
	}

	return version, nil
}

func (repo *Repository) doMigrate(ctx context.Context, conn *pgx.Conn) error {
	fsys, err := fs.Sub(migrations, "migration")
	if err != nil {
		return err
	}

	m, err := migrate.NewMigrator(ctx, conn, "schema_version")
	if err != nil {
		return err
	}

	if err = m.LoadMigrations(fsys); err != nil {
		return err
	}

	m.OnStart = func(seq int32, name string, _ string, _ string) {
		slog.Info(fmt.Sprintf("Starting SQL migration # %d: %s...", seq, name))
	}

	return m.Migrate(ctx)
}

func IsErrNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func NewID() (string, error) {
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return "", err
	}

	return id.String(), nil
}
