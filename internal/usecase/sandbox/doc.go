// Package sandbox holds the one use case behind
// DELETE /api/sandbox/v1/demandes — outside the gateway, outside the ARTP
// contract. Purging touches five tables and must stay atomic, so it is the
// strongest case for port.UnitOfWork in the module; its scope is always
// createur_operateur_id, never the /mes-demandes filter.
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: whether the route is exposed at all. config.SandboxAdmin
// decides that, and internal/framework/web reads it — not this package.
package sandbox
