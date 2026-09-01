package api

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
// one ReverseGateway (Task 16's own addition to port.Repositories), a
// UnitOfWork, two interactors, a presenter and a clock. NewRouter calls it
// once, at router construction, exactly as it does the eight controllers
// before it.
//
// This is the strangler pattern's next stop: internal/api/reverse.go's own
// former self — the handlers that read and wrote through *Deps and
// internal/api/dto.go's Deps.etatNumero directly — is gone, replaced by
// internal/usecase/reverse's two interactors, orchestrating their one write
// through port.UnitOfWork.Do like every other capability's own.
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
