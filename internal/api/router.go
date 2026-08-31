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

	r.POST("/api/authenticate", d.postAuthenticate)
	r.GET("/api/authenticate", d.Authentifier(), d.getAuthenticate)

	// L'authentification est câblée en middleware global, gardé par préfixe,
	// plutôt qu'en middleware de groupe : gin n'exécute le middleware d'un
	// groupe que pour une route qui y est effectivement enregistrée, alors
	// qu'un chemin non contractuel sous /api/gateway/v1 doit renvoyer le même
	// 401 qu'un chemin valide — comme le filtre de sécurité Spring, qui
	// s'exécute avant la résolution de route.
	authentifieGateway := d.Authentifier()
	r.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, prefixeGateway) {
			c.Next()
			return
		}
		authentifieGateway(c)
	})

	g := r.Group(prefixeGateway)
	d.routesReferentiels(g) // Task 6
	d.routesOTP(g)          // Task 7
	d.routesCreation(g)     // Tasks 10-12
	d.routesLecture(g)      // Task 13
	d.routesAcceptation(g)  // Task 14
	d.routesConfirmation(g) // Task 15
	d.routesTraitement(g)   // Task 16
	d.routesAnnulation(g)   // Task 17
	d.routesIncidents(g)    // Task 18
	d.routesReverse(g)      // Task 19

	return r
}
