package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// sandboxController wires the clean-architecture stack behind both
// /api/sandbox/v1 routes. The purge needs a UnitOfWork alone (every gateway
// it touches lives behind port.Repositories.Sandbox, resolved fresh inside
// the transaction); the range count reads through two gateways on the pool,
// the reference one to resolve the operator enum against the operateur
// table and a dedicated one for the aggregate. NewRouter calls this once,
// at router construction, both routes being mounted unconditionally. Moved
// from internal/api/sandbox.go (Task 18).
func (d *Deps) sandboxController() *controller.SandboxController {
	uow := persistence.NewUnitOfWork(d.DB)
	purge := sandbox.NewPurgeTestData(uow)
	ranges := sandbox.NewCountNumberRanges(
		postgres.NewReferenceGateway(d.DB.Pool),
		postgres.NewNumberRangeGateway(d.DB.Pool),
	)
	return controller.NewSandboxController(purge, ranges, d.presenter())
}
