package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/reverse"
)

// reverseController wires the clean-architecture stack behind guide §6's
// two routes — POST /reverse-requests, GET /reverse-requests/mes-demandes —
// one NumberGateway (shared in shape with creationController's own build),
// one ReverseGateway, a UnitOfWork, two interactors, a presenter and a
// clock. NewRouter calls it once, at router construction. No cancellation
// route: the guide excludes it explicitly for a reverse. Moved from
// internal/api/reverse.go (Task 18).
func (d *Deps) reverseController() *controller.ReverseController {
	numbers := postgres.NewNumberGateway(d.DB.Pool)
	reverses := postgres.NewReverseGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	ids := identifier.NewGenerator()
	clk := clock.New(d.Cfg.ClockSkew)

	submit := reverse.NewSubmitReverseRequest(numbers, reverses, uow, ids, clk)
	listOwn := reverse.NewListOwnReverseRequests(reverses)

	return controller.NewReverseController(submit, listOwn, d.presenter(), clk)
}
