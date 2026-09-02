// Package platform holds the part of the NumFlex platform's behaviour that
// no call drives: expiring an overdue step (~349 s, ANO-006), applying a
// deferred convergence (1 to 6 min, R-10), and the reverse lifecycle
// reserved to the ARTP (validation, rejection, completion — §6).
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: the ticker that calls it, nor how often. The loop and
// its schedule live in internal/framework/engine, the only caller of these
// interactors — with cmd/artp calling ValidateReverse and RejectReverse
// for the ARTP's own manual acts.
package platform
