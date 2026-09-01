package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const prefixeGateway = "/api/gateway/v1"

// NewRouter déclare EXACTEMENT les 33 routes du contrat ARTP, plus les deux
// routes d'authentification. Aucune route de santé, de metrics ou de debug :
// le sandbox doit présenter la même surface que la plateforme réelle.
func NewRouter(d *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Avant tout le reste, y compris l'authentification : un préambule CORS part
	// sans jeton. Inerte tant que CORS_ALLOWED_ORIGINS est vide (le défaut), et
	// n'enregistre aucune route — voir cors.go.
	r.Use(d.autoriserCORS())

	// Task 10 : les deux routes /api/authenticate passent en clean
	// architecture — un contrôleur qui délègue à AuthenticateInteractor et
	// DescribeCallerInteractor — et l'authentification elle-même passe du
	// gestionnaire historique Deps.Authentifier au middleware.Authenticate
	// de la Task 6, câblé ici pour la première fois : postgres.UserGateway,
	// la pièce qui lui manquait, apparaît dans authController/authentifier
	// (internal/api/authentification.go). Construits une seule fois, comme
	// otpCtrl plus bas.
	authCtrl := d.authController()
	authentifier := d.authentifier()

	r.POST("/api/authenticate", authCtrl.PostAuthenticate)
	r.GET("/api/authenticate", authentifier, authCtrl.GetAuthenticate)

	// L'authentification est câblée en middleware global, gardé par préfixe,
	// plutôt qu'en middleware de groupe : gin n'exécute le middleware d'un
	// groupe que pour une route qui y est effectivement enregistrée, alors
	// qu'un chemin non contractuel sous /api/gateway/v1 doit renvoyer le même
	// 401 qu'un chemin valide — comme le filtre de sécurité Spring, qui
	// s'exécute avant la résolution de route. La garde compare le segment
	// entier (égalité ou préfixe suivi de "/") pour ne pas capturer un chemin
	// frontalier comme /api/gateway/v1extra ou un futur /api/gateway/v10 —
	// un filtre Spring Security s'écrit "/api/gateway/v1/**", qui exige la
	// barre oblique et ne matcherait jamais ces chemins-là non plus.
	r.Use(func(c *gin.Context) {
		chemin := c.Request.URL.Path
		if chemin != prefixeGateway && !strings.HasPrefix(chemin, prefixeGateway+"/") {
			c.Next()
			return
		}
		authentifier(c)
	})

	g := r.Group(prefixeGateway)

	// Task 11 : les cinq routes de données de référence passent en clean
	// architecture — un contrôleur qui délègue à cinq interactors
	// passe-plat, chacun un simple relais vers ReferenceGateway (aucune
	// règle métier à appliquer à une liste de référence). Construit une
	// seule fois ici, comme otpCtrl et authCtrl.
	refCtrl := d.referenceController()
	g.GET("/operateurs", refCtrl.Operators)
	g.GET("/motifs-rejet", refCtrl.RejectionReasons)
	g.GET("/types-demande", refCtrl.RequestTypes)
	g.GET("/processus", refCtrl.Processes)
	g.GET("/types-incident", refCtrl.IncidentTypes)

	// Task 9 : otp/send et otp/verify passent en clean architecture — le
	// gestionnaire d'autrefois est remplacé par un contrôleur qui ne fait
	// plus rien d'autre que lier la requête, valider la forme du MSISDN et
	// déléguer à l'interactor. Construit une seule fois ici, comme les onze
	// contrôleurs à venir : cmd/server/main.go fournit toujours un *Deps
	// dont DB est un *persistence.DB réel (jamais nil), donc rien n'empêche
	// cette construction d'avoir lieu à la construction du routeur plutôt
	// qu'à chaque requête.
	otpCtrl := d.otpController()
	g.POST("/otp/send", otpCtrl.Send)
	g.POST("/otp/verify", otpCtrl.Verify)

	// Task 12 : les trois routes de création de demande passent en clean
	// architecture — un contrôleur qui délègue à trois interactors, le
	// premier consommateur usecase du port.UnitOfWork de la Task 5.
	// Construit une seule fois ici, comme les quatre contrôleurs précédents.
	creationCtrl := d.creationController()
	g.POST("/demandes/particulier", creationCtrl.Particulier)
	g.POST("/demandes/entreprise", creationCtrl.Entreprise)
	g.POST("/demandes/restitution", creationCtrl.Restitution)

	// Task 12 : les sept files de lecture passent en clean architecture —
	// un contrôleur qui délègue à sept interactors, chacun résolvant des
	// ids via QueryGateway puis leur vue via RequestGateway.Get (déjà
	// construit pour la création). Construit une seule fois ici, comme les
	// cinq contrôleurs précédents.
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

	// Task 14 : les deux routes d'acceptation passent en clean architecture —
	// un contrôleur qui délègue à AcceptRequestInteractor et
	// AcceptFleetRequestInteractor, tous deux au-dessus du même
	// RequestGateway que la création et la lecture. Construit une seule
	// fois ici, comme les six contrôleurs précédents.
	acceptCtrl := d.acceptanceController()
	g.POST("/demandes/acceptation", acceptCtrl.Acceptation)
	g.POST("/demandes/:id/acceptation", acceptCtrl.AcceptationFlotte)

	// Task 15 : confirmation, traitement et annulation passent en clean
	// architecture — un contrôleur qui délègue à trois interactors
	// (internal/usecase/porting), au-dessus du même RequestGateway que les
	// sept contrôleurs précédents, plus ConfirmationGateway pour la seule
	// écriture propre à la confirmation. Construit une seule fois ici,
	// comme les sept contrôleurs précédents.
	portingCtrl := d.portingController()
	g.POST("/demandes/a-confirmer", portingCtrl.Confirmation)
	g.POST("/demandes/traitement", portingCtrl.Traitement)
	g.POST("/demandes/:id/annuler", portingCtrl.Annulation)

	d.routesIncidents(g) // Task 18
	d.routesReverse(g)   // Task 19

	// Hors gateway, et hors contrat ARTP : la purge des données de test. Le
	// groupe n'est monté que si SANDBOX_ADMIN le demande — sinon la route
	// n'existe pas et gin répond 404, sans rien révéler. L'authentification est
	// ici un middleware de groupe et non la garde par préfixe ci-dessus :
	// aucune raison d'imiter le filtre Spring sur une surface qui n'appartient
	// pas à la plateforme, un chemin inconnu sous /api/sandbox/v1 doit donc
	// bien répondre 404 plutôt que 401.
	if d.Cfg.SandboxAdmin {
		d.routesSandbox(r.Group(prefixeSandbox, authentifier))
	}

	return r
}
