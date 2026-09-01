package api

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/incident"
)

// incidentController wires the clean-architecture stack behind guide
// §7.12's six routes — declare, resolve and list, each in a gateway and an
// interne variant — one IncidentGateway (Task 16's own addition to
// port.Repositories), a UnitOfWork, three interactors shared by both
// families, a presenter and a clock. NewRouter calls it once, at router
// construction, exactly as it does the nine controllers before it.
//
// This is the strangler pattern's next stop: internal/api/incidents.go's
// own former self — the handlers that read and wrote through *Deps and its
// own two gin.HandlerFunc-returning closures — is gone, replaced by
// internal/usecase/incident's three interactors, orchestrating their writes
// through port.UnitOfWork.Do like every other capability's own. Neither
// declaring nor resolving an incident checks the frozen-market gate itself
// (acceptance.MarketFrozen, câblé for the confirmation, traitement and
// acceptance routes): a declaration is what causes the freeze, and a
// resolution is what lifts it.
func (d *Deps) incidentController() *controller.IncidentController {
	incidents := postgres.NewIncidentGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	ids := identifier.NewGenerator()
	clk := clock.New(d.Cfg.ClockSkew)

	declare := incident.NewDeclareIncident(incidents, uow, ids, clk)
	resolve := incident.NewResolveIncident(incidents, uow, clk)
	listOwn := incident.NewListOwnIncidents(incidents)

	return controller.NewIncidentController(declare, resolve, listOwn, d.presenter(), clk)
}
