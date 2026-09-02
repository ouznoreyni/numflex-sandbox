package reference

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListIncidentTypesBoundary is the interface a controller drives.
type ListIncidentTypesBoundary interface {
	Execute(context.Context) ([]entity.IncidentType, *entity.Fault)
}

// ListIncidentTypesInteractor implements ListIncidentTypesBoundary.
type ListIncidentTypesInteractor struct {
	gateway port.ReferenceGateway
}

// NewListIncidentTypes wires an interactor against the given gateway.
func NewListIncidentTypes(g port.ReferenceGateway) *ListIncidentTypesInteractor {
	return &ListIncidentTypesInteractor{gateway: g}
}

func (i *ListIncidentTypesInteractor) Execute(ctx context.Context) ([]entity.IncidentType, *entity.Fault) {
	out, err := i.gateway.IncidentTypes(ctx)
	if err != nil {
		return nil, entity.InternalError("reading the incident types")
	}
	return out, nil
}
