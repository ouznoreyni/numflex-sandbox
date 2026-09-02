// Package porting holds the three use cases behind the transitions a
// request goes through once accepted: POST /demandes/a-confirmer (the
// CONFIRMATION step — entity.ExpectedConfirmers decides who),
// POST /demandes/traitement (every other step — entity.CanProcess decides
// who and when) and POST /demandes/:id/annuler (entity.CanCancel decides
// who and when).
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: pgx, Gin, or the etape field a client may still send —
// it is ignored at the edge (ANO-018), never here. Advancing a step is
// port.Engine.ScheduleTransition; this package never computes a deadline
// itself.
package porting
