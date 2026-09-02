package porting

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ProcessStepInput carries POST /demandes/traitement's body, already bound
// by the controller. It carries no étape field — v2 dropped it, and a v1
// client that still sends one is silently ignored rather than rejected
// (ANO-018): the current step is executed regardless of what the caller
// believes it is treating.
type ProcessStepInput struct {
	RequestID string
	Comment   string
}

// ProcessStepBoundary is the interface a controller drives.
type ProcessStepBoundary interface {
	Execute(ctx context.Context, in ProcessStepInput) (port.RequestView, *entity.Fault)
}

// ProcessStepInteractor implements ProcessStepBoundary.
type ProcessStepInteractor struct {
	requests port.RequestGateway
	uow      port.UnitOfWork
	engine   port.Engine
	// completionLatency reproduces ANO-005: COMPLETION is the only step
	// measured slow, ~30.5s twice at SIT. Fixed once at wiring time from
	// config.CompletionLatency — internal/usecase/porting may not import
	// internal/framework/config, so the plain duration crosses at
	// internal/api, exactly as query.NewAlreadyConfirmed's excludeSource
	// bool already does for ANO-019.
	completionLatency time.Duration
}

// NewProcessStep wires an interactor against its dependencies.
func NewProcessStep(
	requests port.RequestGateway, uow port.UnitOfWork, engine port.Engine,
	completionLatency time.Duration,
) *ProcessStepInteractor {
	return &ProcessStepInteractor{
		requests: requests, uow: uow, engine: engine, completionLatency: completionLatency,
	}
}

// Execute reproduces the deleted internal/api/traitement.go's
// postTraitement: entity.CanProcess carries every rule that decides whether
// this caller may process this request's current step right now — the
// REVERSE/ARTP refusal, the in-progress and pending-transition checks, the
// ACCEPTATION/CONFIRMATION redirection to their own endpoints, and the
// source/recipient ownership of every other step. ProcessStep itself does
// only what is left: the ANO-005 sleep, the optional comment write (through
// one port.UnitOfWork.Do, like every other capability's writes), and the
// call to port.Engine.ScheduleTransition that actually moves the request —
// CanProcess decides whether that call may happen, it never performs the
// transition itself.
func (i *ProcessStepInteractor) Execute(
	ctx context.Context, in ProcessStepInput,
) (port.RequestView, *entity.Fault) {
	caller := port.CallerFromContext(ctx)
	dm, found, err := i.requests.ByID(ctx, in.RequestID)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture de la demande")
	}
	if !found {
		return port.RequestView{}, entity.RequestNotFound()
	}
	if f := entity.CanProcess(dm, caller.OperatorID); f != nil {
		return port.RequestView{}, f
	}

	if dm.CurrentStep == entity.StepCompletion && i.completionLatency > 0 {
		time.Sleep(i.completionLatency)
	}

	if in.Comment != "" {
		err := i.uow.Do(ctx, func(repos port.Repositories) error {
			if err := repos.Requests.SetComment(ctx, dm.ID, in.Comment); err != nil {
				return entity.InternalError("enregistrement du commentaire")
			}
			return nil
		})
		if err != nil {
			return port.RequestView{}, entity.FaultFrom(err)
		}
	}

	if err := i.engine.ScheduleTransition(ctx, dm.ID); err != nil {
		return port.RequestView{}, entity.InternalError("planification de la transition")
	}

	// The request is read back AFTER requesting the transition, never after
	// its convergence: R-10 — the response carries the previous step when
	// convergence is deferred, and the next one when it is synchronous,
	// exactly what port.Engine.ScheduleTransition's own implementation
	// decides.
	view, found, err := i.requests.Get(ctx, dm.ID)
	if err != nil || !found {
		return port.RequestView{}, entity.InternalError("relecture de la demande")
	}
	return view, nil
}
