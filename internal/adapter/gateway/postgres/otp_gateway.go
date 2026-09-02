package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// Querier is the narrow slice of *pgxpool.Pool and pgx.Tx that a gateway
// needs. A gateway built on it works unchanged whether it runs directly
// against the pool or inside a transaction opened by the unit of work.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// OTPGateway is the Postgres implementation of port.OTPGateway. Its four
// methods carry the SQL that used to live in internal/api/otp.go, unchanged.
type OTPGateway struct {
	db Querier
}

// NewOTPGateway returns a gateway bound to db — a pool for ad hoc use, or a
// transaction handed out by the unit of work.
func NewOTPGateway(db Querier) *OTPGateway {
	return &OTPGateway{db: db}
}

func (g *OTPGateway) Upsert(ctx context.Context, otp port.OneTimePassword) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO otp (numero, code, expire_a, tentatives, consomme, cree_le)
		 VALUES ($1,$2,$3,0,false,$4)
		 ON CONFLICT (numero) DO UPDATE
		   SET code = EXCLUDED.code, expire_a = EXCLUDED.expire_a,
		       tentatives = 0, consomme = false, cree_le = EXCLUDED.cree_le`,
		otp.MSISDN, otp.Code, otp.ExpiresAt, time.Now())
	return err
}

func (g *OTPGateway) Find(ctx context.Context, msisdn string) (port.OneTimePassword, bool, error) {
	otp := port.OneTimePassword{MSISDN: msisdn}
	err := g.db.QueryRow(ctx,
		`SELECT code, expire_a, tentatives, consomme FROM otp WHERE numero = $1`, msisdn).
		Scan(&otp.Code, &otp.ExpiresAt, &otp.Attempts, &otp.Consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.OneTimePassword{}, false, nil
	}
	if err != nil {
		return port.OneTimePassword{}, false, err
	}
	return otp, true, nil
}

func (g *OTPGateway) IncrementAttempts(ctx context.Context, msisdn string) error {
	_, err := g.db.Exec(ctx,
		`UPDATE otp SET tentatives = tentatives + 1 WHERE numero = $1`, msisdn)
	return err
}

func (g *OTPGateway) Consume(ctx context.Context, msisdn string) error {
	_, err := g.db.Exec(ctx, `UPDATE otp SET consomme = true WHERE numero = $1`, msisdn)
	return err
}
