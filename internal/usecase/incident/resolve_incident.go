package incident

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ResolveIncidentInput carries POST /incidents/{gateway,interne}/:id/resoudre's
// body plus its URL, already bound by the controller. SystemLocked is the
// endpoint's own segment (false: gateway, true: interne), used to detect a
// resolution attempted through the wrong one (§7.12).
type ResolveIncidentInput struct {
	IncidentID   string
	SystemLocked bool
	Comment      string
}

// ResolveIncidentBoundary is the interface a controller drives.
type ResolveIncidentBoundary interface {
	Execute(ctx context.Context, in ResolveIncidentInput) (port.IncidentView, *entity.Fault)
}

// ResolveIncidentInteractor implements ResolveIncidentBoundary.
type ResolveIncidentInteractor struct {
	incidents port.IncidentGateway
	uow       port.UnitOfWork
	clock     port.Clock
}

// NewResolveIncident wires an interactor against its dependencies.
func NewResolveIncident(incidents port.IncidentGateway, uow port.UnitOfWork, clock port.Clock) *ResolveIncidentInteractor {
	return &ResolveIncidentInteractor{incidents: incidents, uow: uow, clock: clock}
}

// Execute reproduces the deleted internal/api/incidents.go's
// resoudreIncident: entity.CanResolveIncident carries both of its rules —
// the segment must match, only the declarant may resolve — then the write
// moves through one port.UnitOfWork.Do like every other capability's
// writes. Resolving an internal incident is what lets
// port.Engine.MarketFrozen answer false again, by the same table read
// DeclareIncidentInteractor's own doc comment explains — nothing here needs
// to call it either.
func (i *ResolveIncidentInteractor) Execute(
	ctx context.Context, in ResolveIncidentInput,
) (port.IncidentView, *entity.Fault) {
	inc, found, err := i.incidents.ByID(ctx, in.IncidentID)
	if err != nil {
		return port.IncidentView{}, entity.InternalError("reading the incident")
	}
	if !found {
		return port.IncidentView{}, entity.IncidentNotFound()
	}

	caller := port.CallerFromContext(ctx)
	if f := entity.CanResolveIncident(inc, in.SystemLocked, caller.OperatorID); f != nil {
		return port.IncidentView{}, f
	}

	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Incidents.Resolve(ctx, in.IncidentID, in.Comment, i.clock.Now()); err != nil {
			return entity.InternalError("resolving the incident")
		}
		return nil
	})
	if err != nil {
		return port.IncidentView{}, entity.FaultFrom(err)
	}

	view, found, err := i.incidents.Get(ctx, in.IncidentID)
	if err != nil || !found {
		return port.IncidentView{}, entity.InternalError("re-reading the incident")
	}
	return view, nil
}
