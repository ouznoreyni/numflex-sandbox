// Package reverse holds the two use cases behind guide §6:
// POST /reverse-requests and GET /reverse-requests/mes-demandes. There is
// no cancellation route — the guide excludes it explicitly ("il n'existe
// pas d'endpoint pour annuler une demande de reverse").
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: the ARTP's own acts on a reverse request. Validation,
// rejection and completion are reserved to the ARTP and live in
// internal/usecase/platform, driven by internal/framework/engine and
// cmd/artp.
package reverse
