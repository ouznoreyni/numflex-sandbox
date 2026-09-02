// Package creation holds the three request-creation use cases behind
// POST /demandes/particulier, POST /demandes/entreprise and
// POST /demandes/restitution — the capability carrying the sandbox's one
// load-bearing transaction: a request insert and the OTP consumption it
// depends on must commit or fail together (commit 643415f). Each
// interactor orchestrates port.NumberGateway and the otp package's
// VerifyOTPBoundary (both read-only, before the transaction), entity's
// eligibility rules, then port.UnitOfWork for the writes.
//
// May import: the standard library, internal/entity,
// internal/usecase/port and internal/usecase/otp.
//
// A creation interactor returns port.RequestView directly rather than a
// package-local copy: port.RequestView already lives in this module's
// use-case layer, so a controller reading it incurs no dependency the
// layering does not already allow.
//
// Must never know: pgx — never a *pgx.Tx, never a SQL string. The
// transaction it needs is expressed as port.UnitOfWork.Do and nothing more.
package creation
