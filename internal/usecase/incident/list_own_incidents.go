package incident

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListOwnIncidentsBoundary is the interface a controller drives for
// GET /incidents/{gateway,interne}/mes-incidents.
type ListOwnIncidentsBoundary interface {
	Execute(ctx context.Context, operatorID string, systemLocked bool, page, size int) ([]port.IncidentView, *entity.Fault)
}

// ListOwnIncidentsInteractor implements ListOwnIncidentsBoundary.
type ListOwnIncidentsInteractor struct {
	incidents port.IncidentGateway
}

// NewListOwnIncidents wires an interactor against its dependency.
func NewListOwnIncidents(incidents port.IncidentGateway) *ListOwnIncidentsInteractor {
	return &ListOwnIncidentsInteractor{incidents: incidents}
}

// Execute reproduces the deleted internal/api/incidents.go's mesIncidents:
// every incident of the caller for this segment, every status, paginated
// (page, size — defaults 0 and 20, like reverse's own list and unlike the
// ten demande queues).
func (i *ListOwnIncidentsInteractor) Execute(
	ctx context.Context, operatorID string, systemLocked bool, page, size int,
) ([]port.IncidentView, *entity.Fault) {
	ids, err := i.incidents.Own(ctx, operatorID, systemLocked, page, size)
	if err != nil {
		return nil, entity.InternalError("lecture des incidents")
	}

	out := make([]port.IncidentView, 0, len(ids))
	for _, id := range ids {
		view, found, err := i.incidents.Get(ctx, id)
		if err != nil || !found {
			return nil, entity.InternalError("lecture de l'incident")
		}
		out = append(out, view)
	}
	return out, nil
}
