package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/reference"
)

// referenceController wires the clean-architecture reference-data stack —
// gateway, five interactors, presenter — behind the five reference routes.
// NewRouter calls it once, at router construction. Moved from
// internal/api/referentiels.go (Task 18).
//
// None of the five interactors carries a business rule: there is nothing to
// validate or filter in a read-only catalog, so each is a direct
// pass-through from gateway to presenter.
func (d *Deps) referenceController() *controller.ReferenceController {
	gw := postgres.NewReferenceGateway(d.DB.Pool)

	operators := reference.NewListOperators(gw)
	rejectionReasons := reference.NewListRejectionReasons(gw)
	requestTypes := reference.NewListRequestTypes(gw)
	processes := reference.NewListProcesses(gw)
	incidentTypes := reference.NewListIncidentTypes(gw)

	return controller.NewReferenceController(
		operators, rejectionReasons, requestTypes, processes, incidentTypes,
		d.presenter(),
	)
}
