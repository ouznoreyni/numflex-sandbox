package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ToAcceptBoundary is the interface a controller drives for
// GET /demandes/a-accepter and its detail route.
type ToAcceptBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
	// Detail answers GET /demandes/a-accepter/:id.
	Detail(ctx context.Context, id, operatorID string) (port.RequestView, *entity.Fault)
}

// ToAcceptInteractor implements ToAcceptBoundary.
type ToAcceptInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
}

// NewToAccept wires an interactor against the given gateways.
func NewToAccept(q port.QueryGateway, r port.RequestGateway) *ToAcceptInteractor {
	return &ToAcceptInteractor{queries: q, requests: r}
}

func (i *ToAcceptInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.ToAccept(ctx, operatorID)
	if err != nil {
		return nil, entity.InternalError("reading the requests to accept")
	}
	return resolveViews(ctx, i.requests, ids)
}

func (i *ToAcceptInteractor) Detail(ctx context.Context, id, operatorID string) (port.RequestView, *entity.Fault) {
	return detailView(ctx, i.queries, i.requests, port.QueueToAccept, id, operatorID,
		"reading the request")
}
