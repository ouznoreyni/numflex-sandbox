package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ConfirmationGateway is the Postgres implementation of
// port.ConfirmationGateway. Confirm carries the INSERT that used to live in
// the deleted internal/api/confirmation.go's postAConfirmer: the anti-replay
// guarantee comes from the confirmation table's own primary key
// (demande_id, operateur_id), not from a pre-check, exactly as that
// handler's own comment explained. This gateway is the one place allowed to
// know that Postgres reports a unique-constraint violation as code 23505 —
// translated here into port.ErrAlreadyConfirmed, since no *pgconn.PgError
// may cross into internal/usecase.
type ConfirmationGateway struct {
	db Querier
}

// NewConfirmationGateway returns a gateway bound to db — a pool for Count's
// post-commit read, or a transaction handed out by the unit of work for
// Confirm's write.
func NewConfirmationGateway(db Querier) *ConfirmationGateway {
	return &ConfirmationGateway{db: db}
}

func (g *ConfirmationGateway) Confirm(ctx context.Context, requestID, operatorID, comment string, now time.Time) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO confirmation (demande_id, operateur_id, commentaire, date_conf)
		 VALUES ($1, $2, NULLIF($3, ''), $4)`,
		requestID, operatorID, comment, now)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return port.ErrAlreadyConfirmed
		}
		return err
	}
	return nil
}

func (g *ConfirmationGateway) Count(ctx context.Context, requestID string) (int, error) {
	var n int
	err := g.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM confirmation WHERE demande_id = $1`, requestID).Scan(&n)
	return n, err
}
