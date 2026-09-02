// Package postgres holds the gateway implementations of the interfaces
// declared in internal/usecase/port, plus mapping.go, the border post
// between the English Go identifiers used from internal/entity upward and
// the French SQL vocabulary that stays frozen.
//
// May import: the standard library, pgx, bcrypt, internal/entity and
// internal/usecase/port.
//
// Must never know: Gin, the fidelity mode, or a business rule — a gateway
// reads and writes, it does not decide. This is the only package in the
// module allowed to name a French table or column, and it never opens its
// own pool: the pgx handle is injected by internal/framework/persistence.
package postgres
