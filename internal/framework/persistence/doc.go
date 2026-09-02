// Package persistence is the driven framework: the pgx pool, the
// golang-migrate runner, and the UnitOfWork implementation that turns
// port.UnitOfWork.Do into a real pgx transaction handing the interactor a
// port.Repositories bound to that transaction.
//
// May import: the standard library, pgx, golang-migrate,
// internal/usecase/port and internal/adapter/gateway/postgres.
//
// Must never know: Gin, a business rule, or which use case is running. It
// is the only package allowed to construct a pgxpool.Pool or a pgx.Tx.
package persistence
