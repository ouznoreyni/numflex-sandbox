package web

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
)

// GatewayPrefix is the root of the 33 routes of the ARTP contract.
const GatewayPrefix = "/api/gateway/v1"

// SandboxPrefix carries what the real platform does not have. It is
// deliberately distinct from GatewayPrefix: the sandbox's promise is that
// /api/gateway/v1 exposes exactly the 33 routes of the ARTP contract, and a
// sandbox convenience must not slip in among them. A client switching its
// baseUrl to the ARTP therefore loses only what it knows does not exist
// there.
const SandboxPrefix = "/api/sandbox/v1"

// GuardGatewayPrefix wires authenticate as a global middleware, guarded by
// prefix, rather than as a group middleware: Gin only runs a group's
// middleware for a route actually registered in that group, but a
// non-contract path under GatewayPrefix must answer the same 401 as a valid
// path — exactly like a Spring Security filter, which runs before route
// resolution rather than after it. The comparison is on whole segments —
// equal to GatewayPrefix, or GatewayPrefix followed by "/" — so that a
// neighbouring path like /api/gateway/v1extra, or a future
// /api/gateway/v10, is not captured: a Spring Security filter is declared
// "/api/gateway/v1/**", which requires the slash and would never match
// those paths either.
func GuardGatewayPrefix(authenticate gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path != GatewayPrefix && !strings.HasPrefix(path, GatewayPrefix+"/") {
			c.Next()
			return
		}
		authenticate(c)
	}
}

// NewRouter declares EXACTLY the 33 routes of the ARTP contract, plus the
// two authentication routes. No health, metrics or debug route: the sandbox
// must present the same surface as the real platform. Moved from
// internal/api/router.go (Task 18): internal/api is deleted, and this
// package — additive since the commit that introduced NewEngine,
// GuardGatewayPrefix and the middleware package — becomes the router
// actually served.
func NewRouter(d *Deps) *gin.Engine {
	// Before everything else, authentication included: a CORS preflight goes
	// out without a token. Inert while CORS_ALLOWED_ORIGINS is empty, and
	// registers no route of its own — see middleware.AllowCORS.
	r := NewEngine(d.Cfg, middleware.AllowCORS(d.Cfg.CORSAllowedOrigins))

	// Task 10: both /api/authenticate routes are a controller delegating to
	// AuthenticateInteractor and DescribeCallerInteractor, and authentication
	// itself is middleware.Authenticate, wired here for the first time:
	// postgres.UserGateway, the piece it was missing, appears in
	// authController/authenticator (auth.go). Built once, like otpCtrl below.
	authCtrl := d.authController()
	authenticate := d.authenticator()

	r.POST("/api/authenticate", authCtrl.PostAuthenticate)
	r.GET("/api/authenticate", authenticate, authCtrl.GetAuthenticate)

	// Authentication is wired as a global middleware guarded by prefix, rather
	// than as a group middleware — see GuardGatewayPrefix's own doc comment.
	r.Use(GuardGatewayPrefix(authenticate))

	g := r.Group(GatewayPrefix)

	// Task 11: the five reference-data routes — a controller delegating to
	// five pass-through interactors, each a plain relay to ReferenceGateway (a
	// reference list has no business rule to apply). Built once here, like
	// otpCtrl and authCtrl.
	refCtrl := d.referenceController()
	g.GET("/operateurs", refCtrl.Operators)
	g.GET("/motifs-rejet", refCtrl.RejectionReasons)
	g.GET("/types-demande", refCtrl.RequestTypes)
	g.GET("/processus", refCtrl.Processes)
	g.GET("/types-incident", refCtrl.IncidentTypes)

	// Task 9: otp/send and otp/verify — a controller that does nothing beyond
	// binding the request, validating the MSISDN's shape and delegating to the
	// interactor. Built once here, as the eleven controllers to come are:
	// cmd/server/main.go always supplies a *Deps whose DB is a real
	// *persistence.DB (never nil), so nothing keeps this construction from
	// happening when the router is built rather than on every request.
	otpCtrl := d.otpController()
	g.POST("/otp/send", otpCtrl.Send)
	g.POST("/otp/verify", otpCtrl.Verify)

	// Task 12: the three request-creation routes — a controller delegating to
	// three interactors, the first use-case consumer of port.UnitOfWork. Built
	// once here, like the four controllers before it.
	creationCtrl := d.creationController()
	g.POST("/demandes/particulier", creationCtrl.Particulier)
	g.POST("/demandes/entreprise", creationCtrl.Entreprise)
	g.POST("/demandes/restitution", creationCtrl.Restitution)

	// Task 12: the seven read queues — a controller delegating to seven
	// interactors, each resolving ids through QueryGateway then their view
	// through RequestGateway.Get (already built for creation). Built once
	// here, like the five controllers before it.
	queryCtrl := d.queryController()
	g.GET("/demandes/mes-demandes", queryCtrl.MesDemandes)
	g.GET("/demandes/a-accepter", queryCtrl.AAccepter)
	g.GET("/demandes/a-accepter/:id", queryCtrl.AAccepterDetail)
	g.GET("/demandes/a-traiter", queryCtrl.ATraiter)
	g.GET("/demandes/a-traiter/:id", queryCtrl.ATraiterDetail)
	g.GET("/demandes/a-confirmer", queryCtrl.AConfirmer)
	g.GET("/demandes/a-confirmer/:id", queryCtrl.AConfirmerDetail)
	g.GET("/demandes/deja-confirmees", queryCtrl.DejaConfirmees)
	g.GET("/demandes/in", queryCtrl.In)
	g.GET("/demandes/out", queryCtrl.Out)

	// Task 14: both acceptance routes — a controller delegating to
	// AcceptRequestInteractor and AcceptFleetRequestInteractor, both above the
	// same RequestGateway creation and reading use. Built once here, like the
	// six controllers before it.
	acceptCtrl := d.acceptanceController()
	g.POST("/demandes/acceptation", acceptCtrl.Acceptation)
	g.POST("/demandes/:id/acceptation", acceptCtrl.AcceptationFlotte)

	// Task 15: confirmation, processing and cancellation — a controller
	// delegating to three interactors (internal/usecase/porting), above the
	// same RequestGateway the seven controllers before it use, plus
	// ConfirmationGateway for the one write specific to confirmation. Built
	// once here, like those seven.
	portingCtrl := d.portingController()
	g.POST("/demandes/a-confirmer", portingCtrl.Confirmation)
	g.POST("/demandes/traitement", portingCtrl.Traitement)
	g.POST("/demandes/:id/annuler", portingCtrl.Annulation)

	// Task 16: the six incident routes (§7.12) — a controller delegating to
	// three interactors (internal/usecase/incident), shared by the gateway and
	// internal families, above an IncidentGateway. Built once here, like the
	// eight controllers before it.
	incidentCtrl := d.incidentController()
	g.POST("/incidents/gateway", incidentCtrl.DeclarerGateway)
	g.POST("/incidents/interne", incidentCtrl.DeclarerInterne)
	g.POST("/incidents/gateway/:id/resoudre", incidentCtrl.ResoudreGateway)
	g.POST("/incidents/interne/:id/resoudre", incidentCtrl.ResoudreInterne)
	g.GET("/incidents/gateway/mes-incidents", incidentCtrl.MesIncidentsGateway)
	g.GET("/incidents/interne/mes-incidents", incidentCtrl.MesIncidentsInterne)

	// Task 16: both reverse routes (§6) — a controller delegating to two
	// interactors (internal/usecase/reverse), above a NumberGateway (the same
	// shape creationController uses) and a ReverseGateway. Built once here,
	// like the nine controllers before it. No cancellation route: the guide
	// excludes it explicitly for a reverse.
	reverseCtrl := d.reverseController()
	g.POST("/reverse-requests", reverseCtrl.Soumission)
	g.GET("/reverse-requests/mes-demandes", reverseCtrl.MesDemandes)

	// Outside the gateway, and outside the ARTP contract: purging the test
	// data. The group is mounted only when SANDBOX_ADMIN asks for it —
	// otherwise the route does not exist and gin answers 404, revealing
	// nothing. Authentication here is a group middleware rather than the
	// prefix guard above: there is no reason to imitate the Spring filter on a
	// surface the platform does not own, so an unknown path under
	// /api/sandbox/v1 should indeed answer 404 rather than 401.
	if d.Cfg.SandboxAdmin {
		sandboxCtrl := d.sandboxController()
		r.Group(SandboxPrefix, authenticate).DELETE("/demandes", sandboxCtrl.Purge)
	}

	return r
}
