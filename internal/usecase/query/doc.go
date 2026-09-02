// Package query holds the seven read-only use cases behind
// GET /demandes/{mes-demandes,a-accepter,a-traiter,a-confirmer,
// deja-confirmees,in,out} — the queues that, in the absence of any callback
// or webhook from NumFlex, are the only way an operator learns a request
// awaits it. Each interactor resolves ids through port.QueryGateway (one
// method per queue) and turns them into views through
// port.RequestGateway.Get, reused rather than duplicated.
//
// ToAccept, ToProcess and ToConfirm carry a second method, Detail, for the
// three queues the guide gives a single-id route.
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: how a view is shaped for the wire. Map building and
// clock skew belong to internal/adapter/controller and
// internal/adapter/presenter, not here — the controller's requestViewDTO
// mirrors CreationController's assembly rather than inventing a third one
// (ruling R28).
package query
