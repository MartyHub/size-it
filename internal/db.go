package internal

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func IsErrNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func ParseUUID(s string) (*pgtype.UUID, error) {
	uuid := new(pgtype.UUID)

	if err := uuid.Scan(s); err != nil {
		return nil, fmt.Errorf("%w: session %s", ErrInvalidInput, s)
	}

	return uuid, nil
}
