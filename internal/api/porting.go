package api

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
// ConfirmationGateway (Task 15's own addition to port.Repositories), a
// UnitOfWork, port.Engine via moteurEngine (acceptation.go), three
// interactors, a presenter and a clock. NewRouter calls it once, at router
// construction, exactly as it does the seven controllers before it.
//
// This is the strangler pattern's next stop: internal/api/confirmation.go,
// traitement.go and annulation.go — the three handlers that read and wrote
// through *Deps and internal/api/dto.go's Deps.demandeDTO directly — are
// gone, replaced by internal/usecase/porting's three interactors, each
// orchestrating its writes through port.UnitOfWork.Do rather than a bare
// pool.Exec or, for annulation, a *pgx.Tx of its own.
func (d *Deps) portingController() *controller.PortingController {
	requests := postgres.NewRequestGateway(d.DB.Pool)
	operators := postgres.NewReferenceGateway(d.DB.Pool)
	confirmations := postgres.NewConfirmationGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	clk := clock.New(d.Cfg.ClockSkew)
	eng := moteurEngine{m: d.Moteur}

	confirm := porting.NewConfirmRequest(requests, operators, confirmations, uow, eng, clk)
	process := porting.NewProcessStep(requests, uow, eng, d.Cfg.CompletionLatency)
	cancel := porting.NewCancelRequest(requests, uow, clk)

	return controller.NewPortingController(confirm, process, cancel, d.presenter(), clk, eng)
}
