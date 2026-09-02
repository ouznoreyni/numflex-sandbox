package persistence

import (
	"context"
	"fmt"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// unitOfWork is the pgx-backed implementation of port.UnitOfWork. It is the
// only place in the module where a pgx.Tx exists.
type unitOfWork struct {
	db *DB
}

// NewUnitOfWork returns a port.UnitOfWork bound to db's pool.
func NewUnitOfWork(db *DB) port.UnitOfWork {
	return &unitOfWork{db: db}
}

// Do opens a transaction, hands fn a port.Repositories whose gateways are
// bound to that transaction, and commits or rolls back depending on how fn
// ends — including when fn panics, which is rolled back and re-raised rather
// than swallowed.
func (u *unitOfWork) Do(ctx context.Context, fn func(port.Repositories) error) (err error) {
	tx, err := u.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("opening the transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		err = tx.Commit(ctx)
	}()

	repos := port.Repositories{
		OTP:           postgres.NewOTPGateway(tx),
		Requests:      postgres.NewRequestGateway(tx),
		Confirmations: postgres.NewConfirmationGateway(tx),
		Reverse:       postgres.NewReverseGateway(tx),
		Incidents:     postgres.NewIncidentGateway(tx),
		Sandbox:       postgres.NewSandboxGateway(tx),
	}
	err = fn(repos)
	return err
}
