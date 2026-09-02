package acceptance

import (
	"context"
	"fmt"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// RejectedNumberInput names one fleet member to reject, and — optionally —
// why: numerosRejetes' entries on POST /demandes/:id/acceptation.
type RejectedNumberInput struct {
	MSISDN            string
	RejectionReasonID string
}

// AcceptFleetRequestInput carries POST /demandes/:id/acceptation's body.
type AcceptFleetRequestInput struct {
	RequestID         string
	Accept            bool
	RejectedNumbers   []RejectedNumberInput
	RejectionReasonID string
	Comment           string
}

// AcceptFleetRequestBoundary is the interface a controller drives.
type AcceptFleetRequestBoundary interface {
	Execute(ctx context.Context, in AcceptFleetRequestInput) (port.RequestView, *entity.Fault)
}

// AcceptFleetRequestInteractor implements AcceptFleetRequestBoundary.
type AcceptFleetRequestInteractor struct {
	requests port.RequestGateway
	reasons  port.ReferenceGateway
	uow      port.UnitOfWork
	engine   port.Engine
	clock    port.Clock
}

// NewAcceptFleetRequest wires an interactor against its dependencies.
func NewAcceptFleetRequest(
	requests port.RequestGateway,
	reasons port.ReferenceGateway,
	uow port.UnitOfWork,
	engine port.Engine,
	clock port.Clock,
) *AcceptFleetRequestInteractor {
	return &AcceptFleetRequestInteractor{
		requests: requests, reasons: reasons, uow: uow, engine: engine, clock: clock,
	}
}

// Execute reproduces the deleted internal/api/acceptation.go's
// postAcceptationFlotte order of checks that are its own to make: the
// request must exist, entity.CanAccept must let this caller decide on it,
// a top-level motifRejetId — given on either branch — must exist. The
// frozen-market check comes first of all, but — as for AcceptRequest — it
// is AcceptanceController's call, made before Execute is even reached; see
// accept_request.go's MarketFrozen doc comment for why.
//
// A total rejection (accepte:false) takes the same path an individual
// rejection does: entity.RequestGateway.Reject inside one transaction, no
// engine transition ever scheduled.
//
// A fleet accept (accepte:true) instead validates every numerosRejetes
// entry before writing anything — each number must belong to this request,
// and its own motif, if given, must exist — exactly as the deleted handler
// did ("everything is validated before anything is written, so as never to
// leave a fleet half marked"). The write itself is one transaction: reject
// each named number, then decide — from inside that same transaction, so
// the check sees the rows it just wrote — whether any number is still
// active. [HYP] The guide never says what becomes of a fleet rejected
// number by number until none is left; neither measured at SIT nor fixed by
// a test. The project's choice, carried over unchanged from the deleted
// handler: such a request has nothing left to port and closes REJETE too,
// with no transition to schedule — the same Reject call a total rejection
// makes. Otherwise the transaction only records the comment, and the engine
// schedules the transition after it commits.
func (i *AcceptFleetRequestInteractor) Execute(
	ctx context.Context, in AcceptFleetRequestInput,
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
		// Total rejection: same handling as an individual request, the whole fleet falls.
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
		return i.readBack(ctx, dm.ID)
	}

	// Partial rejection: each targeted number must belong to the fleet, and
	// its own reason — if it carries one — must exist. Everything is
	// validated before opening the transaction, so as never to leave a
	// fleet half marked.
	for _, nr := range in.RejectedNumbers {
		belongs, err := i.requests.NumberBelongs(ctx, dm.ID, nr.MSISDN)
		if err != nil {
			return port.RequestView{}, entity.InternalError("checking the number")
		}
		if !belongs {
			return port.RequestView{}, entity.ValidationFailed(
				fmt.Sprintf("Le numéro %s ne fait pas partie de cette demande", nr.MSISDN))
		}
		if f := rejectionReasonValid(ctx, i.reasons, nr.RejectionReasonID); f != nil {
			return port.RequestView{}, f
		}
	}

	var fleetExhausted bool
	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		for _, nr := range in.RejectedNumbers {
			if err := repos.Requests.RejectNumber(ctx, dm.ID, nr.MSISDN, nr.RejectionReasonID); err != nil {
				return entity.InternalError("rejecting the number")
			}
		}

		active, err := repos.Requests.HasActiveNumber(ctx, dm.ID)
		if err != nil {
			return entity.InternalError("checking the fleet")
		}
		if !active {
			fleetExhausted = true
			if err := repos.Requests.Reject(ctx, dm.ID, caller.OperatorID, "",
				in.Comment, i.clock.Now()); err != nil {
				return entity.InternalError("rejecting the request")
			}
			return nil
		}

		if err := repos.Requests.SetComment(ctx, dm.ID, in.Comment); err != nil {
			return entity.InternalError("saving the comment")
		}
		return nil
	})
	if err != nil {
		return port.RequestView{}, entity.FaultFrom(err)
	}

	if !fleetExhausted {
		if err := i.engine.ScheduleTransition(ctx, dm.ID); err != nil {
			return port.RequestView{}, entity.InternalError("scheduling the transition")
		}
	}
	return i.readBack(ctx, dm.ID)
}

func (i *AcceptFleetRequestInteractor) readBack(ctx context.Context, id string) (port.RequestView, *entity.Fault) {
	view, found, err := i.requests.Get(ctx, id)
	if err != nil || !found {
		return port.RequestView{}, entity.InternalError("re-reading the request")
	}
	return view, nil
}
