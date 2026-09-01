package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/httpx"
)

type Deps struct {
	Cfg *config.Config
	DB  *persistence.DB
	R   *httpx.Renderer
	// Moteur est renseigné en Task 9. Déclaré ici en interface pour que le
	// paquet api ne dépende pas de l'ordre de livraison des tâches.
	Moteur Moteur
}

// Moteur : la part du comportement de la plateforme que les appels ne pilotent pas.
type Moteur interface {
	PlaceGelee(ctx context.Context) (bool, error)
	PlanifierTransition(ctx context.Context, demandeID string) error
}

// Appelant lit le Caller que middleware.Authenticate (Task 6, câblé en
// Task 10) a résolu et déposé sur le contexte de la requête — pas sur le
// magasin propre à *gin.Context comme le faisait l'ancien Authentifier :
// entity.CallerFromContext est le point de passage commun entre le
// middleware (internal/framework) et ce paquet, qui n'est pas soumis à la
// règle de dépendance de test/architecture_test.go (il n'est ni adapter, ni
// framework) et peut donc lire ce que le middleware y a placé.
func Appelant(c *gin.Context) entity.Caller {
	return entity.CallerFromContext(c.Request.Context())
}
