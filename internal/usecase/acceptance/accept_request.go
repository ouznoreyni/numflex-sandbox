package acceptance

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// MarketFrozen reproduces the deleted internal/api/acceptation.go's
// Deps.verifierGel: a frozen market — BR-012, §6.5 of the guide — blocks
// acceptance, individual or fleet alike. It is exported and called by
// AcceptanceController itself, BEFORE either handler binds its request
// body — the deleted handler's own order, shared by both
// postAcceptation and postAcceptationFlotte behind one doc comment
// presenting it as their universal first gate. A request that arrives
// while the market is frozen AND carries a malformed body must still get
// the frozen-market response, not a JSON-format error: an Execute-level
// check, running only after the controller has already bound the body,
// cannot reproduce that. Fix round 1 (Task 14) moved this out of Execute
// for exactly that reason. Each capability's own controller makes its own
// call rather than sharing this one across packages, since usecase
// packages do not import one another; the same call opens /a-confirmer
// (Task 15) and /traitement (Task 16), per the deleted original's comment.
func MarketFrozen(ctx context.Context, engine port.Engine) *entity.Fault {
	frozen, err := engine.MarketFrozen(ctx)
	if err != nil {
		return entity.InternalError("checking the market-wide freeze")
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
		return entity.InternalError("checking the rejection reason")
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
// postAcceptation order of checks that are its own to make: the request
// must exist, entity.CanAccept must let this caller decide on it, any
// motifRejetId given must exist, and only then the accept/reject branch —
// a rejection requiring its own motif and closing the request inside one
// transaction, an acceptance recording its comment (also inside one
// transaction, for the same reason every write here goes through
// port.UnitOfWork rather than a bare gateway call) before the engine
// schedules its transition. The frozen-market check comes first of all,
// but it is AcceptanceController's call, made before Execute is even
// reached — see MarketFrozen's own doc comment for why.
func (i *AcceptRequestInteractor) Execute(
	ctx context.Context, in AcceptRequestInput,
) (port.RequestView, *entity.Fault) {
	caller := port.CallerFromContext(ctx)
	dm, found, err := i.requests.ByID(ctx, in.RequestID)
	if err != nil {
		return port.RequestView{}, entity.InternalError("reading the request")
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
				return entity.InternalError("rejecting the request")
			}
			return nil
		})
		if err != nil {
			return port.RequestView{}, entity.FaultFrom(err)
		}
	} else {
		err := i.uow.Do(ctx, func(repos port.Repositories) error {
			if err := repos.Requests.SetComment(ctx, dm.ID, in.Comment); err != nil {
				return entity.InternalError("saving the comment")
			}
			return nil
		})
		if err != nil {
			return port.RequestView{}, entity.FaultFrom(err)
		}
		if err := i.engine.ScheduleTransition(ctx, dm.ID); err != nil {
			return port.RequestView{}, entity.InternalError("scheduling the transition")
		}
	}

	view, found, err := i.requests.Get(ctx, dm.ID)
	if err != nil || !found {
		return port.RequestView{}, entity.InternalError("re-reading the request")
	}
	return view, nil
}
