package reverse

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// SubmitReverseRequestInput carries POST /reverse-requests' body, already
// bound and shape-validated (the MSISDN pattern) by the controller.
type SubmitReverseRequestInput struct {
	MSISDN string
}

// SubmitReverseRequestBoundary is the interface a controller drives.
type SubmitReverseRequestBoundary interface {
	Execute(ctx context.Context, in SubmitReverseRequestInput) (port.ReverseView, *entity.Fault)
}

// SubmitReverseRequestInteractor implements SubmitReverseRequestBoundary.
type SubmitReverseRequestInteractor struct {
	numbers port.NumberGateway
	reverse port.ReverseGateway
	uow     port.UnitOfWork
	ids     port.IDGenerator
	clock   port.Clock
}

// NewSubmitReverseRequest wires an interactor against its dependencies.
func NewSubmitReverseRequest(
	numbers port.NumberGateway, reverse port.ReverseGateway,
	uow port.UnitOfWork, ids port.IDGenerator, clock port.Clock,
) *SubmitReverseRequestInteractor {
	return &SubmitReverseRequestInteractor{
		numbers: numbers, reverse: reverse, uow: uow, ids: ids, clock: clock,
	}
}

// Execute reproduces the deleted internal/api/reverse.go's
// postReverseRequest: only the source operator (the number's origin
// operator) may submit, and the number must have been ported at least once
// — otherwise there is nothing to reverse. A number absent from the
// registry cannot belong to the source operator declared, hence
// entity.IncorrectSourceOperator() rather than a not-found fault.
func (i *SubmitReverseRequestInteractor) Execute(
	ctx context.Context, in SubmitReverseRequestInput,
) (port.ReverseView, *entity.Fault) {
	state, found, err := i.numbers.State(ctx, in.MSISDN)
	if err != nil {
		return port.ReverseView{}, entity.InternalError("reading the number")
	}
	if !found {
		return port.ReverseView{}, entity.IncorrectSourceOperator()
	}

	caller := port.CallerFromContext(ctx)
	if state.OriginOperatorID != caller.OperatorID {
		return port.ReverseView{}, entity.RequestAccessDenied(
			"Seul l'opérateur source (opérateur d'origine du numéro) peut soumettre " +
				"une demande de reverse pour ce numéro.")
	}
	if state.CurrentOperatorID == state.OriginOperatorID {
		return port.ReverseView{}, entity.NumberNotPorted()
	}

	id := i.ids.NewID()
	now := i.clock.Now()
	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Reverse.Create(ctx, port.ReverseCreateInput{
			ID: id, MSISDN: in.MSISDN, OperatorID: caller.OperatorID, RequestDate: now,
		}); err != nil {
			return entity.InternalError("creating the reverse request")
		}
		return nil
	})
	if err != nil {
		return port.ReverseView{}, entity.FaultFrom(err)
	}

	view, found, err := i.reverse.Get(ctx, id)
	if err != nil || !found {
		return port.ReverseView{}, entity.InternalError("re-reading the reverse request")
	}
	return view, nil
}
