package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/query"
)

// queryController wires the clean-architecture read-models stack — one
// QueryGateway, one RequestGateway (shared with creationController's own
// build: both are pool-bound and stateless, so a second construction here
// costs nothing and keeps this file independent of creation.go), seven
// interactors, a presenter and a clock — behind the seven read-only routes.
// NewRouter calls it once, at router construction. Moved from
// internal/api/query.go (Task 18).
//
// excludeSource — ANO-019 — is resolved here, at the one point in this
// package allowed to read config.Fidelity, and handed to
// query.NewAlreadyConfirmed as a plain bool: neither
// internal/usecase/query nor internal/adapter/gateway/postgres may import
// internal/framework/config.
func (d *Deps) queryController() *controller.QueryController {
	gw := postgres.NewQueryGateway(d.DB.Pool)
	requestsRead := postgres.NewRequestGateway(d.DB.Pool)
	clk := clock.New(d.Cfg.ClockSkew)
	excludeSource := d.Cfg.Fidelity == config.FidelityReal

	own := query.NewOwn(gw, requestsRead)
	toAccept := query.NewToAccept(gw, requestsRead)
	toProcess := query.NewToProcess(gw, requestsRead)
	toConfirm := query.NewToConfirm(gw, requestsRead)
	alreadyConfirmed := query.NewAlreadyConfirmed(gw, requestsRead, excludeSource)
	incoming := query.NewIncoming(gw, requestsRead)
	outgoing := query.NewOutgoing(gw, requestsRead)

	return controller.NewQueryController(
		own, toAccept, toProcess, toConfirm, alreadyConfirmed, incoming, outgoing,
		d.presenter(), clk,
	)
}
