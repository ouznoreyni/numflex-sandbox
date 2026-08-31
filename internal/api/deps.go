package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/httpx"
	"github.com/yas/numflex-sandbox/internal/store"
)

type Deps struct {
	Cfg *config.Config
	DB  *store.DB
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

// Identite décrit l'opérateur derrière le jeton présenté.
type Identite struct {
	UtilisateurID string
	Username      string
	OperateurID   string
	OperateurNom  string
}

const cleIdentite = "numflex.identite"

func Appelant(c *gin.Context) Identite {
	v, _ := c.Get(cleIdentite)
	id, _ := v.(Identite)
	return id
}
