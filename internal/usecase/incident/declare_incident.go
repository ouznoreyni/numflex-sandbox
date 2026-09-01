// Package incident holds the three use cases behind guide §7.12's six
// routes — two families, gateway and interne, sharing the same logic
// parameterized by systemLocked (fige_systeme), the only dimension where
// they really diverge: the interne segment alone carries the rule "one open
// internal incident per operator", and freezes the market
// (port.Engine.MarketFrozen simply reads the incident table these
// interactors write, so nothing here needs to drive that state directly —
// see DeclareIncidentInteractor's own doc comment).
package incident

import (
	"context"
	"errors"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// DeclareIncidentInput carries POST /incidents/{gateway,interne}'s body,
// already bound by the controller. SystemLocked comes from the endpoint's
// own segment, never the body — §7.12 decodes no typeIncidentId: a caller
// that sends one is silently ignored, exactly as the deleted
// internal/api/incidents.go's reqIncident already documented.
type DeclareIncidentInput struct {
	SystemLocked bool
	Comment      string
}

// DeclareIncidentBoundary is the interface a controller drives.
type DeclareIncidentBoundary interface {
	Execute(ctx context.Context, in DeclareIncidentInput) (port.IncidentView, *entity.Fault)
}

// DeclareIncidentInteractor implements DeclareIncidentBoundary.
type DeclareIncidentInteractor struct {
	incidents port.IncidentGateway
	uow       port.UnitOfWork
	ids       port.IDGenerator
	clock     port.Clock
}

// NewDeclareIncident wires an interactor against its dependencies.
func NewDeclareIncident(
	incidents port.IncidentGateway, uow port.UnitOfWork, ids port.IDGenerator, clock port.Clock,
) *DeclareIncidentInteractor {
	return &DeclareIncidentInteractor{incidents: incidents, uow: uow, ids: ids, clock: clock}
}

// Execute reproduces the deleted internal/api/incidents.go's
// declarerIncident: resolve the type matching this segment, refuse a second
// EN_COURS internal incident for the same operator (§7.12, interne only —
// this pre-check only anticipates a clean business message; the migration's
// own partial unique index, translated by the gateway into
// port.ErrIncidentAlreadyOpen, is the real guarantee against the race), then
// write the row through one port.UnitOfWork.Do like every other
// capability's writes. This declaration is itself what later makes
// port.Engine.MarketFrozen answer true — BR-012 — since that check simply
// counts EN_COURS/fige_systeme rows in the same table; nothing here needs
// to call it.
func (i *DeclareIncidentInteractor) Execute(
	ctx context.Context, in DeclareIncidentInput,
) (port.IncidentView, *entity.Fault) {
	typeID, err := i.incidents.TypeIDFor(ctx, in.SystemLocked)
	if err != nil {
		return port.IncidentView{}, entity.InternalError("résolution du type d'incident")
	}

	caller := port.CallerFromContext(ctx)

	if in.SystemLocked {
		open, err := i.incidents.HasOpen(ctx, caller.OperatorID)
		if err != nil {
			return port.IncidentView{}, entity.InternalError("vérification des incidents ouverts")
		}
		if open {
			return port.IncidentView{}, entity.InvalidStep(
				"Un incident interne est déjà ouvert pour votre opérateur.")
		}
	}

	id := i.ids.NewID()
	now := i.clock.Now()
	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		err := repos.Incidents.Create(ctx, port.IncidentCreateInput{
			ID: id, OperatorID: caller.OperatorID, TypeID: typeID,
			SystemLocked: in.SystemLocked, Description: in.Comment, OpenedAt: now,
		})
		if errors.Is(err, port.ErrIncidentAlreadyOpen) {
			return entity.InvalidStep("Un incident interne est déjà ouvert pour votre opérateur.")
		}
		if err != nil {
			return entity.InternalError("déclaration de l'incident")
		}
		return nil
	})
	if err != nil {
		return port.IncidentView{}, entity.FaultFrom(err)
	}

	view, found, err := i.incidents.Get(ctx, id)
	if err != nil || !found {
		return port.IncidentView{}, entity.InternalError("relecture de l'incident")
	}
	return view, nil
}
