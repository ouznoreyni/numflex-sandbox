package porting

import (
	"context"
	"errors"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// CancelRequestBoundary is the interface a controller drives for
// POST /demandes/:id/annuler.
type CancelRequestBoundary interface {
	Execute(ctx context.Context, requestID string) (port.RequestView, *entity.Fault)
}

// CancelRequestInteractor implements CancelRequestBoundary.
type CancelRequestInteractor struct {
	requests port.RequestGateway
	uow      port.UnitOfWork
	clock    port.Clock
}

// NewCancelRequest wires an interactor against its dependencies.
func NewCancelRequest(requests port.RequestGateway, uow port.UnitOfWork, clock port.Clock) *CancelRequestInteractor {
	return &CancelRequestInteractor{requests: requests, uow: uow, clock: clock}
}

// Execute reproduces the deleted internal/api/annulation.go's postAnnuler:
// entity.CanCancel carries both of its rules — only the creator operator,
// only still at ACCEPTATION — and the two writes (a terminal
// etape_historique row, then the demande row itself) move from a bare
// *pgx.Tx into one port.UnitOfWork.Do, exactly as every other capability's
// state change already does. No frozen-market gate here: [HYP], preserved
// unchanged from the deleted handler's own comment — cancelling a request
// still at ACCEPTATION withdraws it rather than processing a step, and
// BR-012 only blocks processing.
//
// dm.CurrentStep — the step entity.CanCancel just authorized against — is
// passed to Cancel so its own guard checks the very thing CanCancel checked,
// rather than Cancel re-reading the request itself: a second read would
// reopen the same gap a scheduled convergence can land in between this
// read and the transaction's write (Task 17b). When that guard loses the
// race, Cancel returns port.ErrCancelStepChanged and the caller receives
// entity.InvalidStep — the same fault CanCancel itself would give for a
// request no longer at a cancellable step — rather than a silent no-op.
func (i *CancelRequestInteractor) Execute(
	ctx context.Context, requestID string,
) (port.RequestView, *entity.Fault) {
	caller := port.CallerFromContext(ctx)
	dm, found, err := i.requests.ByID(ctx, requestID)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture de la demande")
	}
	if !found {
		return port.RequestView{}, entity.RequestNotFound()
	}
	if f := entity.CanCancel(dm, caller.OperatorID); f != nil {
		return port.RequestView{}, f
	}

	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		err := repos.Requests.Cancel(ctx, dm.ID, caller.OperatorID, dm.CurrentStep, i.clock.Now())
		if errors.Is(err, port.ErrCancelStepChanged) {
			return entity.InvalidStep(
				"Cette demande ne peut plus être annulée : son étape a changé depuis l'autorisation.")
		}
		if err != nil {
			return entity.InternalError("annulation de la demande")
		}
		return nil
	})
	if err != nil {
		return port.RequestView{}, entity.FaultFrom(err)
	}

	view, found, err := i.requests.Get(ctx, dm.ID)
	if err != nil || !found {
		return port.RequestView{}, entity.InternalError("relecture de la demande")
	}
	return view, nil
}
