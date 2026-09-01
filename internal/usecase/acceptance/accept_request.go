// Package acceptance holds the two use cases behind
// POST /demandes/acceptation and POST /demandes/:id/acceptation — the
// individual/restitution accept-or-reject decision and its fleet
// counterpart, which can reject some fleet numbers while accepting the
// rest. Both interactors share the same shape: a frozen market blocks
// either (BR-012, routed through port.Engine.MarketFrozen rather than
// reimplemented here), entity.CanAccept already holds the sole
// authorization rule (only the source operator, only at ACCEPTATION,
// neither interactor restates it), and a rejection's writes go through
// exactly one port.UnitOfWork.Do, as request creation's already do.
package acceptance

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// marketFrozen reproduces the deleted internal/api/acceptation.go's
// Deps.verifierGel: a frozen market — BR-012, §6.5 of the guide — blocks
// acceptance, individual or fleet alike. The comment on the original noted
// the same call would open /a-confirmer (Task 15) and /traitement
// (Task 16); each capability's own interactor makes its own call to
// port.Engine.MarketFrozen rather than sharing this function across
// packages, since usecase packages do not import one another.
func marketFrozen(ctx context.Context, engine port.Engine) *entity.Fault {
	frozen, err := engine.MarketFrozen(ctx)
	if err != nil {
		return entity.InternalError("vérification du gel de la place")
	}
	if frozen {
		return entity.InternalError(
			"Le traitement des demandes est gelé par un incident interne en cours.")
	}
	return nil
}

// rejectionReasonValid checks a caller-supplied motifRejetId that is not
// empty against the referential — the guard the deleted acceptation.go's
// motifExiste applied unconditionally, whether the demande ends up accepted
// or rejected. An empty id is valid here (nothing to check); the rejection
// path's own requirement that a rejection *carry* one is a separate rule,
// applied by each interactor's reject branch.
func rejectionReasonValid(ctx context.Context, reasons port.ReferenceGateway, id string) *entity.Fault {
	if id == "" {
		return nil
	}
	exists, err := reasons.RejectionReasonExists(ctx, id)
	if err != nil {
		return entity.InternalError("vérification du motif de rejet")
	}
	if !exists {
		return entity.ValidationFailed("Motif de rejet inconnu")
	}
	return nil
}

// AcceptRequestInput carries POST /demandes/acceptation's body, already
// bound by the controller. It serves both a particulier and a restitution
// request: entity.CanAccept applies the same rule to either type.
type AcceptRequestInput struct {
	RequestID         string
	Accept            bool
	RejectionReasonID string
	Comment           string
}

// AcceptRequestBoundary is the interface a controller drives.
type AcceptRequestBoundary interface {
	Execute(ctx context.Context, in AcceptRequestInput) (port.RequestView, *entity.Fault)
}

// AcceptRequestInteractor implements AcceptRequestBoundary.
type AcceptRequestInteractor struct {
	requests port.RequestGateway
	reasons  port.ReferenceGateway
	uow      port.UnitOfWork
	engine   port.Engine
	clock    port.Clock
}

// NewAcceptRequest wires an interactor against its dependencies.
func NewAcceptRequest(
	requests port.RequestGateway,
	reasons port.ReferenceGateway,
	uow port.UnitOfWork,
	engine port.Engine,
	clock port.Clock,
) *AcceptRequestInteractor {
	return &AcceptRequestInteractor{
		requests: requests, reasons: reasons, uow: uow, engine: engine, clock: clock,
	}
}

// Execute reproduces the deleted internal/api/acceptation.go's
// postAcceptation order of checks: the market must not be frozen, the
// request must exist, entity.CanAccept must let this caller decide on it,
// any motifRejetId given must exist, and only then the accept/reject
// branch — a rejection requiring its own motif and closing the request
// inside one transaction, an acceptance recording its comment (also inside
// one transaction, for the same reason every write here goes through
// port.UnitOfWork rather than a bare gateway call) before the engine
// schedules its transition.
func (i *AcceptRequestInteractor) Execute(
	ctx context.Context, in AcceptRequestInput,
) (port.RequestView, *entity.Fault) {
	if f := marketFrozen(ctx, i.engine); f != nil {
		return port.RequestView{}, f
	}

	caller := port.CallerFromContext(ctx)
	dm, found, err := i.requests.ByID(ctx, in.RequestID)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture de la demande")
	}
	if !found {
		return port.RequestView{}, entity.RequestNotFound()
	}
	if f := entity.CanAccept(dm, caller.OperatorID); f != nil {
		return port.RequestView{}, f
	}
	if f := rejectionReasonValid(ctx, i.reasons, in.RejectionReasonID); f != nil {
		return port.RequestView{}, f
	}

	if !in.Accept {
		if in.RejectionReasonID == "" {
			return port.RequestView{}, entity.RejectionReasonRequired()
		}
		err := i.uow.Do(ctx, func(repos port.Repositories) error {
			if err := repos.Requests.Reject(ctx, dm.ID, caller.OperatorID,
				in.RejectionReasonID, in.Comment, i.clock.Now()); err != nil {
				return entity.InternalError("rejet de la demande")
			}
			return nil
		})
		if err != nil {
			return port.RequestView{}, entity.FaultFrom(err)
		}
	} else {
		err := i.uow.Do(ctx, func(repos port.Repositories) error {
			if err := repos.Requests.SetComment(ctx, dm.ID, in.Comment); err != nil {
				return entity.InternalError("enregistrement du commentaire")
			}
			return nil
		})
		if err != nil {
			return port.RequestView{}, entity.FaultFrom(err)
		}
		if err := i.engine.ScheduleTransition(ctx, dm.ID); err != nil {
			return port.RequestView{}, entity.InternalError("planification de la transition")
		}
	}

	view, found, err := i.requests.Get(ctx, dm.ID)
	if err != nil || !found {
		return port.RequestView{}, entity.InternalError("relecture de la demande")
	}
	return view, nil
}
