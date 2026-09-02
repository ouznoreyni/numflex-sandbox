package query

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// AlreadyConfirmedBoundary is the interface a controller drives for
// GET /demandes/deja-confirmees. It has no detail route in the guide.
type AlreadyConfirmedBoundary interface {
	Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault)
}

// AlreadyConfirmedInteractor implements AlreadyConfirmedBoundary.
type AlreadyConfirmedInteractor struct {
	queries  port.QueryGateway
	requests port.RequestGateway
	// excludeSource is ANO-019, fixed once at wiring time from
	// config.Fidelity — see port.QueryGateway.AlreadyConfirmed.
	excludeSource bool
}

// NewAlreadyConfirmed wires an interactor against the given gateways and
// fidelity-mode setting.
func NewAlreadyConfirmed(q port.QueryGateway, r port.RequestGateway, excludeSource bool) *AlreadyConfirmedInteractor {
	return &AlreadyConfirmedInteractor{queries: q, requests: r, excludeSource: excludeSource}
}

func (i *AlreadyConfirmedInteractor) Execute(ctx context.Context, operatorID string) ([]port.RequestView, *entity.Fault) {
	ids, err := i.queries.AlreadyConfirmed(ctx, operatorID, i.excludeSource)
	if err != nil {
		return nil, entity.InternalError("reading the already-confirmed requests")
	}
	return resolveViews(ctx, i.requests, ids)
}
