package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS n'appartient pas au contrat ARTP : la gateway réelle est consommée de
// serveur à serveur, et aucun test du SIT n'a mesuré son comportement sur une
// requête cross-origin. Ce middleware n'existe donc que pour le confort du bac
// à sable — permettre à la page Swagger, servie sur un autre port, d'appeler
// l'API depuis un navigateur. Il est inerte tant que CORS_ALLOWED_ORIGINS est
// vide, ce qui est le défaut.
//
// Il est délibérément écrit comme un middleware global plutôt qu'en routes
// OPTIONS enregistrées : la contrainte D-4 veut que le sandbox n'expose que les
// 33 routes du contrat, et une route OPTIONS par endpoint doublerait sa surface.
func (d *Deps) autoriserCORS() gin.HandlerFunc {
	origines := d.Cfg.CORSAllowedOrigins
	toutes := false
	for _, o := range origines {
		if o == "*" {
			toutes = true
		}
	}

	return func(c *gin.Context) {
		origine := c.GetHeader("Origin")
		if origine == "" || len(origines) == 0 {
			c.Next()
			return
		}

		autorisee := toutes
		for _, o := range origines {
			if o == origine {
				autorisee = true
				break
			}
		}
		if !autorisee {
			// Aucun en-tête : le navigateur bloquera la lecture de la réponse,
			// ce qui est le comportement attendu d'une origine non autorisée.
			c.Next()
			return
		}

		valeur := origine
		if toutes {
			valeur = "*"
		}
		c.Header("Access-Control-Allow-Origin", valeur)
		c.Header("Vary", "Origin")

		// Préambule : le navigateur l'émet sans en-tête Authorization. Il doit
		// être soldé ici, avant le middleware d'authentification, sinon il part
		// en 401 et la vraie requête n'est jamais émise.
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", strings.Join([]string{
				http.MethodGet, http.MethodPost, http.MethodOptions,
			}, ", "))
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
