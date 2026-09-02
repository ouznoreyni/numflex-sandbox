package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const prefixeGateway = "/api/gateway/v1"

// NewRouter declares EXACTLY the 33 routes of the ARTP contract, plus the two
// authentication routes. No health, metrics or debug route: the sandbox must
// present the same surface as the real platform.
func NewRouter(d *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Before everything else, authentication included: a CORS preflight goes out
	// without a token. Inert while CORS_ALLOWED_ORIGINS is empty, and registers
	// no route of its own — see cors.go.
	r.Use(d.autoriserCORS())

	// Task 10: both /api/authenticate routes move to clean architecture — a
	// controller delegating to AuthenticateInteractor and
	// DescribeCallerInteractor — and authentication itself moves from the
	// legacy Deps.Authentifier handler to Task 6's middleware.Authenticate,
	// wired here for the first time: postgres.UserGateway, the piece it was
	// missing, appears in authController/authentifier
	// (internal/api/authentification.go). Built once, like otpCtrl below.
	authCtrl := d.authController()
	authentifier := d.authentifier()

	r.POST("/api/authenticate", authCtrl.PostAuthenticate)
	r.GET("/api/authenticate", authentifier, authCtrl.GetAuthenticate)

	// Authentication is wired as a global middleware guarded by prefix, rather
	// than as a group middleware: gin runs a group's middleware only for a route
	// actually registered in it, whereas a non-contractual path under
	// /api/gateway/v1 must answer the same 401 a valid path does — like the
	// Spring security filter, which runs before route resolution. The guard
	// compares whole segments (equality, or the prefix followed by "/") so as
	// not to catch a bordering path such as /api/gateway/v1extra or a future
	// /api/gateway/v10 — a Spring Security filter is written "/api/gateway/v1/**",
	// which demands the slash and would never match those paths either.
	r.Use(func(c *gin.Context) {
		chemin := c.Request.URL.Path
		if chemin != prefixeGateway && !strings.HasPrefix(chemin, prefixeGateway+"/") {
			c.Next()
			return
		}
		authentifier(c)
	})

	g := r.Group(prefixeGateway)

	// Task 11: the five reference-data routes move to clean architecture — a
	// controller delegating to five pass-through interactors, each a plain relay
	// to ReferenceGateway (a reference list has no business rule to apply).
	// Built once here, like otpCtrl and authCtrl.
	refCtrl := d.referenceController()
	g.GET("/operateurs", refCtrl.Operators)
	g.GET("/motifs-rejet", refCtrl.RejectionReasons)
	g.GET("/types-demande", refCtrl.RequestTypes)
	g.GET("/processus", refCtrl.Processes)
	g.GET("/types-incident", refCtrl.IncidentTypes)

	// Task 9: otp/send and otp/verify move to clean architecture — the old
	// handler gives way to a controller that does nothing beyond binding the
	// request, validating the MSISDN's shape and delegating to the interactor.
	// Built once here, as the eleven controllers to come will be:
	// cmd/server/main.go always supplies a *Deps whose DB is a real
	// *persistence.DB (never nil), so nothing keeps this construction from
	// happening when the router is built rather than on every request.
	otpCtrl := d.otpController()
	g.POST("/otp/send", otpCtrl.Send)
	g.POST("/otp/verify", otpCtrl.Verify)

	// Task 12: the three request-creation routes move to clean architecture — a
	// controller delegating to three interactors, the first use-case consumer of
	// Task 5's port.UnitOfWork. Built once here, like the four controllers
	// before it.
	creationCtrl := d.creationController()
	g.POST("/demandes/particulier", creationCtrl.Particulier)
	g.POST("/demandes/entreprise", creationCtrl.Entreprise)
	g.POST("/demandes/restitution", creationCtrl.Restitution)

	// Task 12: the seven read queues move to clean architecture — a controller
	// delegating to seven interactors, each resolving ids through QueryGateway
	// then their view through RequestGateway.Get (already built for creation).
	// Built once here, like the five controllers before it.
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

	// Task 14: both acceptance routes move to clean architecture — a controller
	// delegating to AcceptRequestInteractor and AcceptFleetRequestInteractor,
	// both above the same RequestGateway creation and reading use. Built once
	// here, like the six controllers before it.
	acceptCtrl := d.acceptanceController()
	g.POST("/demandes/acceptation", acceptCtrl.Acceptation)
	g.POST("/demandes/:id/acceptation", acceptCtrl.AcceptationFlotte)

	// Task 15: confirmation, processing and cancellation move to clean
	// architecture — a controller delegating to three interactors
	// (internal/usecase/porting), above the same RequestGateway the seven
	// controllers before it use, plus ConfirmationGateway for the one write
	// specific to confirmation. Built once here, like those seven.
	portingCtrl := d.portingController()
	g.POST("/demandes/a-confirmer", portingCtrl.Confirmation)
	g.POST("/demandes/traitement", portingCtrl.Traitement)
	g.POST("/demandes/:id/annuler", portingCtrl.Annulation)

	// Task 16: the six incident routes (§7.12) move to clean architecture — a
	// controller delegating to three interactors (internal/usecase/incident),
	// shared by the gateway and internal families, above an IncidentGateway.
	// Built once here, like the eight controllers before it.
	incidentCtrl := d.incidentController()
	g.POST("/incidents/gateway", incidentCtrl.DeclarerGateway)
	g.POST("/incidents/interne", incidentCtrl.DeclarerInterne)
	g.POST("/incidents/gateway/:id/resoudre", incidentCtrl.ResoudreGateway)
	g.POST("/incidents/interne/:id/resoudre", incidentCtrl.ResoudreInterne)
	g.GET("/incidents/gateway/mes-incidents", incidentCtrl.MesIncidentsGateway)
	g.GET("/incidents/interne/mes-incidents", incidentCtrl.MesIncidentsInterne)

	// Task 16: both reverse routes (§6) move to clean architecture — a controller
	// delegating to two interactors (internal/usecase/reverse), above a
	// NumberGateway (the same shape creationController uses) and a
	// ReverseGateway. Built once here, like the nine controllers before it. No
	// cancellation route: the guide excludes it explicitly for a reverse.
	reverseCtrl := d.reverseController()
	g.POST("/reverse-requests", reverseCtrl.Soumission)
	g.GET("/reverse-requests/mes-demandes", reverseCtrl.MesDemandes)

	// Outside the gateway, and outside the ARTP contract: purging the test data.
	// The group is mounted only when SANDBOX_ADMIN asks for it — otherwise the
	// route does not exist and gin answers 404, revealing nothing. Authentication
	// here is a group middleware rather than the prefix guard above: there is no
	// reason to imitate the Spring filter on a surface the platform does not own,
	// so an unknown path under /api/sandbox/v1 should indeed answer 404 rather
	// than 401.
	if d.Cfg.SandboxAdmin {
		sandboxCtrl := d.sandboxController()
		r.Group(prefixeSandbox, authentifier).DELETE("/demandes", sandboxCtrl.Purge)
	}

	return r
}
