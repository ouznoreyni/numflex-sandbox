package reference

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListRequestTypesBoundary is the interface a controller drives.
type ListRequestTypesBoundary interface {
	Execute(context.Context) ([]entity.RequestTypeRef, *entity.Fault)
}

// ListRequestTypesInteractor implements ListRequestTypesBoundary.
type ListRequestTypesInteractor struct {
	gateway port.ReferenceGateway
}

// NewListRequestTypes wires an interactor against the given gateway.
func NewListRequestTypes(g port.ReferenceGateway) *ListRequestTypesInteractor {
	return &ListRequestTypesInteractor{gateway: g}
}

func (i *ListRequestTypesInteractor) Execute(ctx context.Context) ([]entity.RequestTypeRef, *entity.Fault) {
	out, err := i.gateway.RequestTypes(ctx)
	if err != nil {
		return nil, entity.InternalError("reading the request types")
	}
	return out, nil
}
