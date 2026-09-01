package api

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/reference"
)

// referenceController wires the clean-architecture reference-data stack —
// gateway, five interactors, presenter — behind the five reference routes.
// NewRouter calls it once, at router construction, exactly as it does
// otpController (internal/api/otp.go) and authController
// (internal/api/authentification.go): the same build-once rationale applies
// unchanged.
//
// None of the five interactors carries a business rule: there is nothing to
// validate or filter in a read-only catalog, so each is a direct
// pass-through from gateway to presenter — see internal/usecase/reference's
// package doc for why that is correct rather than under-engineered.
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
