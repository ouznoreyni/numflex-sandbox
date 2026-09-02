package platform

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ConvergePendingTransitionsInteractor applies every deferred transition
// whose deadline has passed (R-10). Moved in substance from the deleted
// internal/engine/engine.go's own appliquerConvergencesDues.
type ConvergePendingTransitionsInteractor struct {
	requests port.RequestGateway
	uow      port.UnitOfWork
	clock    port.Clock
}

// NewConvergePendingTransitions wires an interactor against its
// dependencies.
func NewConvergePendingTransitions(requests port.RequestGateway, uow port.UnitOfWork, clock port.Clock) *ConvergePendingTransitionsInteractor {
	return &ConvergePendingTransitionsInteractor{requests: requests, uow: uow, clock: clock}
}

// Execute reproduces appliquerConvergencesDues: every request whose
// transition_prevue_a has come due, per port.RequestGateway.DueConvergences'
// own database-side comparison, converges — origin "ACTION".
func (i *ConvergePendingTransitionsInteractor) Execute(ctx context.Context) error {
	ids, err := i.requests.DueConvergences(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := ApplyTransition(ctx, i.uow, i.clock, id, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}
