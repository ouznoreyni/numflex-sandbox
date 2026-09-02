package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ToProcessBoundary is the interface a controller drives for
// GET /demandes/a-traiter and its detail route.
type ToProcessBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
	// Detail answers GET /demandes/a-traiter/:id.
	Detail(ctx context.Context, id, operatorID string) (port.RequestView, *entity.Fault)
}

// ToProcessInteractor implements ToProcessBoundary.
type ToProcessInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
}

// NewToProcess wires an interactor against the given gateways.
func NewToProcess(q port.QueryGateway, r port.RequestGateway) *ToProcessInteractor {
	return &ToProcessInteractor{queries: q, requests: r}
}

func (i *ToProcessInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.ToProcess(ctx, operatorID)
	if err != nil {
		return nil, entity.InternalError("reading the requests to process")
	}
	return resolveViews(ctx, i.requests, ids)
}

func (i *ToProcessInteractor) Detail(ctx context.Context, id, operatorID string) (port.RequestView, *entity.Fault) {
	return detailView(ctx, i.queries, i.requests, port.QueueToProcess, id, operatorID,
		"reading the request")
}
