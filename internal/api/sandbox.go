package api

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// prefixeSandbox carries what the real platform does not have. It is
// deliberately distinct from prefixeGateway: the sandbox's promise is that
// /api/gateway/v1 exposes exactly the 33 routes of the ARTP contract, and a
// sandbox convenience must not slip in among them. A client switching its
// baseUrl to the ARTP therefore loses only what it knows does not exist there.
const prefixeSandbox = "/api/sandbox/v1"

// sandboxController wires the clean-architecture stack behind
// DELETE /api/sandbox/v1/demandes — a UnitOfWork alone, the one dependency
// PurgeTestDataInteractor needs (every gateway it touches lives behind
// port.Repositories.Sandbox, resolved fresh inside the transaction), and a
// presenter. NewRouter calls it once, but only when config.SandboxAdmin is
// true — see NewRouter's own comment for why an unknown path under this
// prefix answers 404 rather than 401 either way.
//
// This is the strangler pattern's next stop: internal/api/sandbox.go's own
// former self — deletePurgeDemandes, which opened its own *pgx.Tx for five
// tables — is gone, replaced by internal/usecase/sandbox's single
// interactor, the strongest case for port.UnitOfWork in the whole project.
func (d *Deps) sandboxController() *controller.SandboxController {
	uow := persistence.NewUnitOfWork(d.DB)
	purge := sandbox.NewPurgeTestData(uow)
	return controller.NewSandboxController(purge, d.presenter())
}
