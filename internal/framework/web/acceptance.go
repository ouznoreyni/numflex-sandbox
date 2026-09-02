package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/acceptance"
)

// acceptanceController wires the clean-architecture acceptance stack — one
// RequestGateway (shared in shape with creationController's and
// queryController's own builds), one ReferenceGateway for the
// motifRejetId guard, a UnitOfWork, d.Engine directly as port.Engine — its
// MarketFrozen/ScheduleTransition methods already match that port's shape,
// so no adapter is needed here — two interactors, a presenter and a clock —
// behind the two acceptance routes. NewRouter calls it once, at router
// construction. Moved from internal/api/acceptation.go (Task 18).
func (d *Deps) acceptanceController() *controller.AcceptanceController {
	requests := postgres.NewRequestGateway(d.DB.Pool)
	reasons := postgres.NewReferenceGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	clk := clock.New(d.Cfg.ClockSkew)
	eng := d.Engine

	individual := acceptance.NewAcceptRequest(requests, reasons, uow, eng, clk)
	fleet := acceptance.NewAcceptFleetRequest(requests, reasons, uow, eng, clk)

	return controller.NewAcceptanceController(individual, fleet, d.presenter(), clk, eng)
}
