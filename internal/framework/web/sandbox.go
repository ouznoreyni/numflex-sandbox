package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// sandboxController wires the clean-architecture stack behind
// DELETE /api/sandbox/v1/demandes — a UnitOfWork alone, the one dependency
// PurgeTestDataInteractor needs (every gateway it touches lives behind
// port.Repositories.Sandbox, resolved fresh inside the transaction), and a
// presenter. NewRouter calls it, but only when config.SandboxAdmin is true
// — see NewRouter's own comment for why an unknown path under this prefix
// answers 404 rather than 401 either way. Moved from internal/api/sandbox.go
// (Task 18).
func (d *Deps) sandboxController() *controller.SandboxController {
	uow := persistence.NewUnitOfWork(d.DB)
	purge := sandbox.NewPurgeTestData(uow)
	return controller.NewSandboxController(purge, d.presenter())
}
