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
			return nil, entity.InternalError("reading the request")
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
	_, found, err := queries.ByID(ctx, queue, id, operatorID)
	if err != nil {
		return port.RequestView{}, entity.InternalError(errMsg)
	}
	if !found {
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
		return nil, entity.InternalError("reading the requests")
	}
	return resolveViews(ctx, i.requests, ids)
}
