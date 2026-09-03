// Package sandbox holds the two use cases behind /api/sandbox/v1 — outside
// the gateway, outside the ARTP contract. Purging touches five tables and
// must stay atomic, so it is the strongest case for port.UnitOfWork in the
// module; its scope is always createur_operateur_id, never the
// /mes-demandes filter. Counting the registry's ranges only reads, through
// one aggregate.
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: how the routes are mounted, or what they cost in
// latency. internal/framework/web decides the first, the gateway measures
// the second — not this package.
package sandbox
