package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
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

const cleIdentite = "numflex.identite"

func Appelant(c *gin.Context) entity.Caller {
	v, _ := c.Get(cleIdentite)
	id, _ := v.(entity.Caller)
	return id
}
