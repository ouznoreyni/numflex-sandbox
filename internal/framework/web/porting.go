package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/porting"
)

// portingController wires the clean-architecture stack behind the three
// routes a request goes through once accepted — POST /demandes/a-confirmer,
// POST /demandes/traitement, POST /demandes/:id/annuler — one RequestGateway
// (shared in shape with creationController's, queryController's and
// acceptanceController's own builds), one ReferenceGateway for the "every
// operator on the market" list entity.ExpectedConfirmers needs, a
// ConfirmationGateway, a UnitOfWork, d.Engine directly as port.Engine (see
// acceptanceController's own comment for why no adapter is needed), three
// interactors, a presenter and a clock. NewRouter calls it once, at router
// construction. Moved from internal/api/porting.go (Task 18).
func (d *Deps) portingController() *controller.PortingController {
	requests := postgres.NewRequestGateway(d.DB.Pool)
	operators := postgres.NewReferenceGateway(d.DB.Pool)
	confirmations := postgres.NewConfirmationGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	clk := clock.New(d.Cfg.ClockSkew)
	eng := d.Engine

	confirm := porting.NewConfirmRequest(requests, operators, confirmations, uow, eng, clk)
	process := porting.NewProcessStep(requests, uow, eng, d.Cfg.CompletionLatency)
	cancel := porting.NewCancelRequest(requests, uow, clk)

	return controller.NewPortingController(confirm, process, cancel, d.presenter(), clk, eng)
}
