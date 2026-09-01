// Package query holds the seven read-only use cases behind
// GET /demandes/{mes-demandes,a-accepter,a-traiter,a-confirmer,
// deja-confirmees,in,out} — internal/api/demandes_lecture.go before this
// task. Each interactor resolves ids through port.QueryGateway (the
// per-queue filter, one method per queue) and turns them into views through
// port.RequestGateway.Get — already built for request creation, reused
// rather than duplicated — returning []port.RequestView for the controller
// to render. The map-building and clock skew that used to happen inline in
// Deps.demandeDTO is the controller's job now, not this package's: see
// internal/adapter/controller/query_controller.go's requestViewDTO, which
// mirrors CreationController's assembly rather than inventing a third one
// (ruling R28).
//
// ToAccept, ToProcess and ToConfirm carry a second method, Detail, for the
// three queues with a single-id route (/a-accepter/:id, /a-traiter/:id,
// /a-confirmer/:id) — Own, AlreadyConfirmed, Incoming and Outgoing have none
// in the guide.
package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// resolveViews turns a list of request ids into their renderable views. out
// starts as []port.RequestView{}, never nil, so a queue with zero matching
// ids still marshals to [] downstream — never null — exactly as
// Deps.rendreListe was written to guarantee (its ids argument, and its own
// out, both started non-nil before the loop that could fail partway).
func resolveViews(ctx context.Context, requests port.RequestGateway, ids []string) ([]port.RequestView, *entity.Fault) {
	out := []port.RequestView{}
	for _, id := range ids {
		view, found, err := requests.Get(ctx, id)
		if err != nil || !found {
			return nil, entity.InternalError("lecture de la demande")
		}
		out = append(out, view)
	}
	return out, nil
}

// detailView resolves the single-id membership check a detail route needs
// (port.QueryGateway.ByID) and then the same RequestGateway.Get list
// methods use. entity.RequestNotFound covers both "id doesn't exist" and
// "id exists but isn't in this queue for this caller" — the legacy
// handlers' detailFiltre/detailParmi never told the two apart either.
func detailView(ctx context.Context, queries port.QueryGateway, requests port.RequestGateway,
	queue port.Queue, id, operatorID, errMsg string) (port.RequestView, *entity.Fault) {
	_, trouve, err := queries.ByID(ctx, queue, id, operatorID)
	if err != nil {
		return port.RequestView{}, entity.InternalError(errMsg)
	}
	if !trouve {
		return port.RequestView{}, entity.RequestNotFound()
	}
	view, found, err := requests.Get(ctx, id)
	if err != nil {
		return port.RequestView{}, entity.InternalError(errMsg)
	}
	if !found {
		return port.RequestView{}, entity.RequestNotFound()
	}
	return view, nil
}

// OwnBoundary is the interface a controller drives for
// GET /demandes/mes-demandes.
type OwnBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
}

// OwnInteractor implements OwnBoundary.
type OwnInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
}

// NewOwn wires an interactor against the given gateways.
func NewOwn(q port.QueryGateway, r port.RequestGateway) *OwnInteractor {
	return &OwnInteractor{queries: q, requests: r}
}

func (i *OwnInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.Own(ctx, operatorID)
	if err != nil {
		return nil, entity.InternalError("lecture des demandes")
	}
	return resolveViews(ctx, i.requests, ids)
}
