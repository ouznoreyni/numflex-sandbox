package platform

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ExpireOverdueStepsInteractor implements ANO-006: a step left untouched
// past STEP_TIMEOUT_SECONDS expires on its own, with no operator call.
// Moved in substance from the deleted internal/engine/engine.go's own
// expirerEtapes.
type ExpireOverdueStepsInteractor struct {
	requests port.RequestGateway
	uow      port.UnitOfWork
	clock    port.Clock
	timeout  time.Duration
}

// NewExpireOverdueSteps wires an interactor against its dependencies.
// timeout is STEP_TIMEOUT_SECONDS (internal/framework/config.Config), a
// plain time.Duration since this package may not import
// internal/framework/config — the same boundary
// internal/usecase/porting.ProcessStepInteractor's own completionLatency
// already crosses.
func NewExpireOverdueSteps(requests port.RequestGateway, uow port.UnitOfWork, clock port.Clock, timeout time.Duration) *ExpireOverdueStepsInteractor {
	return &ExpireOverdueStepsInteractor{requests: requests, uow: uow, clock: clock, timeout: timeout}
}

// Execute reproduces expirerEtapes: asOf is the single instant the whole
// tick shares — internal/framework/engine.Engine.Tick's own doc comment
// explains why (a request that converges with a short StepTimeout must not
// re-match this predicate within the same tick). A zero timeout disables
// expiration entirely, exactly as before.
func (i *ExpireOverdueStepsInteractor) Execute(ctx context.Context, asOf time.Time) error {
	if i.timeout <= 0 {
		return nil
	}
	ids, err := i.requests.OverdueSteps(ctx, i.timeout.Seconds(), asOf)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := ApplyTransition(ctx, i.uow, i.clock, id, "EXPIRATION"); err != nil {
			return err
		}
	}
	return nil
}
