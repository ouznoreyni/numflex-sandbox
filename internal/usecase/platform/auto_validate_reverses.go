package platform

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ValidateReverse is an act of the ARTP, outside the API gateway's scope
// (§6). It creates a REVERSE-type Request directly at the CONFIRMATION
// step: neither ACCEPTATION, nor DESACTIVATION/ACTIVATION. Moved in
// substance from the deleted internal/engine/reverse.go's own
// ValidateReverse: internal/framework/engine.ValidateReverse (cmd/artp's own
// caller) and AutoValidateReversesInteractor's own auto-validation loop
// both call this same function, so the ARTP's manual act and the sandbox's
// automatic stand-in for it never drift apart.
func ValidateReverse(ctx context.Context, uow port.UnitOfWork, ids port.IDGenerator, clock port.Clock, reverseID string) error {
	return uow.Do(ctx, func(repos port.Repositories) error {
		msisdn, operatorID, status, err := repos.Reverse.LockPending(ctx, reverseID)
		if err != nil {
			return err
		}
		if status != "EN_ATTENTE" {
			return nil
		}

		currentHolder, err := repos.Reverse.CurrentOperatorFor(ctx, msisdn)
		if err != nil {
			return err
		}

		id := ids.NewID()
		now := clock.Now()
		if err := repos.Requests.CreateAtConfirmation(ctx, port.CreateRequestInput{
			ID:                  id,
			MSISDN:              msisdn,
			SubscriberType:      string(entity.SubscriberIndividual),
			RequestType:         string(entity.RequestTypeReverse),
			SourceOperatorID:    currentHolder,
			RecipientOperatorID: operatorID,
			CreatorOperatorID:   operatorID,
			RequestDate:         now,
		}); err != nil {
			return err
		}
		if err := repos.Requests.AddNumber(ctx, port.RequestNumberInput{
			RequestID: id, MSISDN: msisdn,
		}); err != nil {
			return err
		}
		return repos.Reverse.MarkValidated(ctx, reverseID, id, now)
	})
}

// RejectReverse is also an act of the ARTP: reject the request without ever
// creating a Request. Moved verbatim in substance from the deleted
// internal/engine/reverse.go's own RejectReverse.
func RejectReverse(ctx context.Context, uow port.UnitOfWork, reverseID string) error {
	return uow.Do(ctx, func(repos port.Repositories) error {
		return repos.Reverse.Reject(ctx, reverseID)
	})
}

// AutoValidateReversesInteractor covers the reverse lifecycle's two
// tick-driven behaviours, in the fixed order internal/engine.Tick already
// ran them in: automatic validation of an overdue reverse request, then
// automatic completion of a confirmed one. Moved in substance from the
// deleted internal/engine/reverse.go's own validerReversesAutomatiquement
// and completerReversesConfirmes.
type AutoValidateReversesInteractor struct {
	reverse             port.ReverseGateway
	requests            port.RequestGateway
	uow                 port.UnitOfWork
	ids                 port.IDGenerator
	clock               port.Clock
	autoValidationDelay time.Duration
}

// NewAutoValidateReverses wires an interactor against its dependencies.
// autoValidationDelay is REVERSE_AUTO_VALIDATION_SECONDS
// (internal/framework/config.Config), a plain time.Duration crossing the
// same boundary ExpireOverdueStepsInteractor's own timeout already does.
func NewAutoValidateReverses(
	reverse port.ReverseGateway, requests port.RequestGateway, uow port.UnitOfWork,
	ids port.IDGenerator, clock port.Clock, autoValidationDelay time.Duration,
) *AutoValidateReversesInteractor {
	return &AutoValidateReversesInteractor{
		reverse: reverse, requests: requests, uow: uow, ids: ids, clock: clock,
		autoValidationDelay: autoValidationDelay,
	}
}

// Execute runs the reverse lifecycle's two steps in order — see this type's
// own doc comment for why the order is fixed.
func (i *AutoValidateReversesInteractor) Execute(ctx context.Context) error {
	if err := i.autoValidate(ctx); err != nil {
		return err
	}
	return i.autoComplete(ctx)
}

// autoValidate replays ValidateReverse for every request EN_ATTENTE for
// longer than REVERSE_AUTO_VALIDATION_SECONDS. Disabled by default (0 =
// never): in the real world, validation is a human act of the ARTP,
// outside the API; this delay exists only to let the sandbox simulate the
// regulator's approval without CLI intervention.
func (i *AutoValidateReversesInteractor) autoValidate(ctx context.Context) error {
	if i.autoValidationDelay <= 0 {
		return nil
	}
	ids, err := i.reverse.OverdueForAutoValidation(ctx, i.autoValidationDelay.Seconds())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := ValidateReverse(ctx, i.uow, i.ids, i.clock, id); err != nil {
			return err
		}
	}
	return nil
}

// autoComplete: a REVERSE's COMPLETION is reserved to the ARTP. No endpoint
// exposes it; it is the engine that pronounces it once every operator has
// confirmed.
//
// This function must also catch up a REVERSE request already at COMPLETION:
// postAConfirmer is agnostic of the request type, and when the last
// confirmation lands, it schedules a generic transition via
// port.Engine.ScheduleTransition. On the next tick, the generic convergence
// runs before this function and moves the request from CONFIRMATION to
// COMPLETION through the common path, resetting transition_prevue_a to
// NULL — and since a REVERSE's COMPLETION belongs to no operator, no
// endpoint can then advance it further. Without this catch-up, the request
// would stay stuck at COMPLETION/EN_COURS forever. The CONFIRMATION branch
// stays necessary: it applies when autoValidate brings a request this far
// without ever going through postAConfirmer (every confirmation may have
// been recorded before the last one triggers the scheduling, or the
// request may not yet have any transition scheduled).
func (i *AutoValidateReversesInteractor) autoComplete(ctx context.Context) error {
	candidates, err := i.requests.PendingReverseCompletions(ctx)
	if err != nil {
		return err
	}
	for _, c := range candidates {
		// From CONFIRMATION: CONFIRMATION → COMPLETION, then COMPLETION →
		// TERMINE. From COMPLETION (already reached by the generic
		// convergence): a single transition suffices, COMPLETION → TERMINE.
		if c.CurrentStep == entity.StepConfirmation {
			if err := ApplyTransition(ctx, i.uow, i.clock, c.RequestID, "ACTION"); err != nil {
				return err
			}
		}
		if err := ApplyTransition(ctx, i.uow, i.clock, c.RequestID, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}
