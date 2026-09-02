package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// IncomingBoundary is the interface a controller drives for
// GET /demandes/in. It has no detail route in the guide.
type IncomingBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
}

// IncomingInteractor implements IncomingBoundary.
type IncomingInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
}

// NewIncoming wires an interactor against the given gateways.
func NewIncoming(q port.QueryGateway, r port.RequestGateway) *IncomingInteractor {
	return &IncomingInteractor{queries: q, requests: r}
}

func (i *IncomingInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.Incoming(ctx, operatorID)
	if err != nil {
		return nil, entity.InternalError("reading the IN requests")
	}
	return resolveViews(ctx, i.requests, ids)
}
