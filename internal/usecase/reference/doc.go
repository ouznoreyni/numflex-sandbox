// Package reference holds the five read-only reference-data use cases
// behind /operateurs, /motifs-rejet, /types-demande, /processus and
// /types-incident. There is no business rule to apply to a reference list —
// no validation to invent, no filtering, no defensive re-checking of what
// the gateway returned — so each interactor is a deliberate pass-through:
// one port.ReferenceGateway call, one gateway error turned into an
// *entity.Fault. None of them is padded with logic that isn't there.
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: SQL. The five SELECT statements live in
// internal/adapter/gateway/postgres/reference_gateway.go.
package reference
