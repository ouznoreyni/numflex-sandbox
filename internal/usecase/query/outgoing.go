package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// OutgoingBoundary is the interface a controller drives for
// GET /demandes/out. It has no detail route in the guide.
type OutgoingBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
}

// OutgoingInteractor implements OutgoingBoundary.
type OutgoingInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
}

// NewOutgoing wires an interactor against the given gateways.
func NewOutgoing(q port.QueryGateway, r port.RequestGateway) *OutgoingInteractor {
	return &OutgoingInteractor{queries: q, requests: r}
}

func (i *OutgoingInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.Outgoing(ctx, operatorID)
	if err != nil {
		return nil, entity.InternalError("reading the OUT requests")
	}
	return resolveViews(ctx, i.requests, ids)
}
