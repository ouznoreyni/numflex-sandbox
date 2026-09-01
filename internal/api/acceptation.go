package api

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/acceptance"
)

// moteurEngine adapts Deps.Moteur — PlaceGelee/PlanifierTransition, the
// vocabulary internal/engine.Engine and its own tests have always used — to
// port.Engine's MarketFrozen/ScheduleTransition: same behaviour, renamed
// for the use-case layer. This file is the only wiring point where the
// mismatch needs bridging; nothing upstream of it (internal/engine, cmd/
// server's ticker) has any reason to know the use-case layer's vocabulary
// exists.
type moteurEngine struct{ m Moteur }

func (e moteurEngine) MarketFrozen(ctx context.Context) (bool, error) {
	return e.m.PlaceGelee(ctx)
}

func (e moteurEngine) ScheduleTransition(ctx context.Context, requestID string) error {
	return e.m.PlanifierTransition(ctx, requestID)
}

// acceptanceController wires the clean-architecture acceptance stack — one
// RequestGateway (shared in shape with creationController's and
// queryController's own builds), one ReferenceGateway for the
// motifRejetId guard, a UnitOfWork, port.Engine via moteurEngine above, two
// interactors, a presenter and a clock — behind the two acceptance routes.
// NewRouter calls it once, at router construction, exactly as it does the
// six controllers before it.
//
// This is the strangler pattern's next stop: internal/api/acceptation.go —
// the 334-line handler that opened a *pgx.Tx directly for a fleet's
// numéro-by-numéro rejection — is gone, replaced by
// internal/usecase/acceptance's two interactors, each orchestrating a
// single port.UnitOfWork.Do rather than any *pgx.Tx of its own.
func (d *Deps) acceptanceController() *controller.AcceptanceController {
	requests := postgres.NewRequestGateway(d.DB.Pool)
	reasons := postgres.NewReferenceGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	clk := clock.New(d.Cfg.ClockSkew)
	eng := moteurEngine{m: d.Moteur}

	individual := acceptance.NewAcceptRequest(requests, reasons, uow, eng, clk)
	fleet := acceptance.NewAcceptFleetRequest(requests, reasons, uow, eng, clk)

	return controller.NewAcceptanceController(individual, fleet, d.presenter(), clk)
}
