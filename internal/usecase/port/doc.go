// Package port declares what the use case layer needs from the outside
// world: the ten gateway interfaces every interactor calls, the UnitOfWork
// that turns several gateway writes into one transaction, the injected
// services (Clock, IDGenerator, Engine) and the view structs a controller
// reads back. It is the seam that lets an interactor be tested against an
// in-memory double and run against Postgres unchanged.
//
// May import: the standard library and internal/entity.
//
// Must never know: pgx, Gin, or any other concrete technology — an
// interface here names a capability, never an implementation. The
// implementations live in internal/adapter/gateway/postgres and
// internal/framework/persistence; this package must not name them either,
// since they sit further out.
package port
