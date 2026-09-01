package api

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// prefixeSandbox porte ce que la plateforme réelle n'a pas. Il est délibérément
// distinct de prefixeGateway : la promesse du sandbox est que /api/gateway/v1
// expose exactement les 33 routes du contrat ARTP, et une commodité de bac à
// sable ne doit pas s'y glisser. Un client qui bascule son baseUrl vers l'ARTP
// ne perd donc que ce qu'il sait ne pas exister là-bas.
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
