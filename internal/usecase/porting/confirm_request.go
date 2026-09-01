// Package porting holds the three use cases behind the transitions a
// request goes through once accepted: POST /demandes/a-confirmer (the
// CONFIRMATION step, shared by every operator on the market bar an
// occasional recipient — entity.ExpectedConfirmers decides which),
// POST /demandes/traitement (every other step — entity.CanProcess decides
// who and when) and POST /demandes/:id/annuler (withdrawing a request that
// has not yet moved — entity.CanCancel decides who and when). All three sit
// downstream of request creation and acceptance, over the same
// port.RequestGateway those already share.
package porting

import (
	"context"
	"errors"
	"fmt"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ConfirmRequestInput carries POST /demandes/a-confirmer's body, already
// bound by the controller.
type ConfirmRequestInput struct {
	RequestID string
	Comment   string
}

// ConfirmRequestBoundary is the interface a controller drives.
type ConfirmRequestBoundary interface {
	Execute(ctx context.Context, in ConfirmRequestInput) (port.RequestView, *entity.Fault)
}

// ConfirmRequestInteractor implements ConfirmRequestBoundary.
type ConfirmRequestInteractor struct {
	requests      port.RequestGateway
	operators     port.ReferenceGateway
	confirmations port.ConfirmationGateway
	uow           port.UnitOfWork
	engine        port.Engine
	clock         port.Clock
}

// NewConfirmRequest wires an interactor against its dependencies. operators
// is the same port.ReferenceGateway acceptance already carries — reused
// here for Operators(), the "every operator on the market" list
// entity.ExpectedConfirmers needs, rather than a dedicated method: the
// deleted internal/api/dto.go's own tousOperateurs read the same operateur
// table this one does.
func NewConfirmRequest(
	requests port.RequestGateway,
	operators port.ReferenceGateway,
	confirmations port.ConfirmationGateway,
	uow port.UnitOfWork,
	engine port.Engine,
	clock port.Clock,
) *ConfirmRequestInteractor {
	return &ConfirmRequestInteractor{
		requests: requests, operators: operators, confirmations: confirmations,
		uow: uow, engine: engine, clock: clock,
	}
}

// Execute reproduces the deleted internal/api/confirmation.go's
// postAConfirmer: measured at SIT, a PORTAGE settles once every operator on
// the market except the recipient has confirmed — the recipient
// auto-confirmed once the others have — while a RESTITUTION or a REVERSE
// requires everyone, recipient included. entity.ExpectedConfirmers is the
// single place that rule lives, called here exactly as
// internal/usecase/query's ToConfirm calls it for the /a-confirmer queue, so
// the two can never diverge. The anti-replay guarantee comes from the
// confirmation table's own primary key (demande_id, operateur_id), enforced
// by port.ConfirmationGateway.Confirm — not by a pre-check here, which would
// race two concurrent calls from the same operator.
func (i *ConfirmRequestInteractor) Execute(
	ctx context.Context, in ConfirmRequestInput,
) (port.RequestView, *entity.Fault) {
	caller := port.CallerFromContext(ctx)
	dm, found, err := i.requests.ByID(ctx, in.RequestID)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture de la demande")
	}
	if !found {
		return port.RequestView{}, entity.RequestNotFound()
	}
	if dm.Status != entity.RequestInProgress || dm.CurrentStep != entity.StepConfirmation {
		return port.RequestView{}, entity.InvalidStep(fmt.Sprintf(
			"Cette demande n'est pas à l'étape CONFIRMATION (étape actuelle : %s).", dm.CurrentStep))
	}
	if dm.PendingTransition {
		return port.RequestView{}, entity.InvalidStep(
			"L'étape CONFIRMATION a déjà été soldée pour cette demande.")
	}

	allOperators, err := i.operators.Operators(ctx)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture des opérateurs")
	}
	ids := make([]string, 0, len(allOperators))
	for _, op := range allOperators {
		ids = append(ids, op.ID)
	}
	expected := entity.ExpectedConfirmers(dm, ids)

	attendu := false
	for _, op := range expected {
		if op == caller.OperatorID {
			attendu = true
			break
		}
	}
	if !attendu {
		return port.RequestView{}, entity.RequestAccessDenied(
			"Votre opérateur n'a pas à confirmer cette demande.")
	}

	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		err := repos.Confirmations.Confirm(ctx, dm.ID, caller.OperatorID, in.Comment, i.clock.Now())
		if errors.Is(err, port.ErrAlreadyConfirmed) {
			return entity.RequestAccessDenied("Votre opérateur a déjà confirmé cette demande.")
		}
		if err != nil {
			return entity.InternalError("enregistrement de la confirmation")
		}
		return nil
	})
	if err != nil {
		return port.RequestView{}, entity.FaultFrom(err)
	}

	count, err := i.confirmations.Count(ctx, dm.ID)
	if err != nil {
		return port.RequestView{}, entity.InternalError("comptage des confirmations")
	}
	if count >= len(expected) {
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
