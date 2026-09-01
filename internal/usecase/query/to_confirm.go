package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ToConfirmBoundary is the interface a controller drives for
// GET /demandes/a-confirmer and its detail route. Neither method removes
// the client sub-object from the views it returns — sansClient is a JSON
// concern applied by the controller, not something a port.RequestView can
// express (its Client field is typed, not a map to delete a key from).
type ToConfirmBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
	// Detail answers GET /demandes/a-confirmer/:id.
	Detail(ctx context.Context, id, operatorID string) (port.RequestView, *entity.Fault)
}

// ToConfirmInteractor implements ToConfirmBoundary.
type ToConfirmInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
}

// NewToConfirm wires an interactor against the given gateways.
func NewToConfirm(q port.QueryGateway, r port.RequestGateway) *ToConfirmInteractor {
	return &ToConfirmInteractor{queries: q, requests: r}
}

func (i *ToConfirmInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.ToConfirm(ctx, operatorID)
	if err != nil {
		return nil, entity.InternalError("lecture des demandes à confirmer")
	}
	return resolveViews(ctx, i.requests, ids)
}

func (i *ToConfirmInteractor) Detail(ctx context.Context, id, operatorID string) (port.RequestView, *entity.Fault) {
	return detailView(ctx, i.queries, i.requests, port.QueueToConfirm, id, operatorID,
		"lecture de la demande")
}
