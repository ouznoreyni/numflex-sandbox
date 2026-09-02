// Package acceptance holds the two use cases behind
// POST /demandes/acceptation and POST /demandes/:id/acceptation — the
// individual/restitution accept-or-reject decision and its fleet
// counterpart, which can reject some fleet numbers while accepting the
// rest (BR-006). Both interactors share the same shape:
// entity.CanAccept already holds the sole authorization rule (only the
// source operator, only at ACCEPTATION, neither interactor restates it),
// and a rejection's writes go through exactly one port.UnitOfWork.Do.
// The frozen-market gate (BR-012) is deliberately NOT one of Execute's own
// checks — MarketFrozen is exported and called by the controller before it
// binds a body, so a malformed request arriving on a frozen market still
// gets the frozen-market answer.
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: pgx, Gin, SQL, or the fidelity mode. It reaches the
// database only through port gateways, and its outcome is a plain output
// model a presenter later renders.
package acceptance
