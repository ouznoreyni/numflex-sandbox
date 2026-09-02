package inmemory

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// UnitOfWork is a non-transactional port.UnitOfWork double: Do simply calls
// fn against the fixed Repositories it was built with. There is no real
// rollback here — an in-memory gateway double writes unconditionally, so
// "does a failed fn really leave nothing behind" is not a question this
// double can answer. That guarantee is proven separately, against real
// Postgres, by internal/framework/persistence's own transaction tests.
type UnitOfWork struct {
	repos port.Repositories
}

// NewUnitOfWork returns a double whose Do always hands fn the same repos.
func NewUnitOfWork(repos port.Repositories) *UnitOfWork {
	return &UnitOfWork{repos: repos}
}

func (u *UnitOfWork) Do(_ context.Context, fn func(port.Repositories) error) error {
	return fn(u.repos)
}
