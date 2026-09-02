package web

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
// interne variant — one IncidentGateway, a UnitOfWork, three interactors
// shared by both families, a presenter and a clock. NewRouter calls it
// once, at router construction. Moved from internal/api/incidents.go
// (Task 18).
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
