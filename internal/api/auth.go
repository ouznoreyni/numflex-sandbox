package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/auth"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/httpx"
	"golang.org/x/crypto/bcrypt"
)

type demandeAuth struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

func (d *Deps) postAuthenticate(c *gin.Context) {
	var req demandeAuth
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, entity.InvalidJSONFormat())
		return
	}

	var hash string
	var roles []string
	err := d.DB.Pool.QueryRow(c, `SELECT password_hash, roles FROM utilisateur WHERE username = $1`,
		req.Username).Scan(&hash, &roles)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		// ANO-016 : hors enveloppe, en problem+json JHipster.
		c.Header("Content-Type", "application/problem+json")
		c.JSON(http.StatusUnauthorized, gin.H{
			"type":    "https://www.jhipster.tech/problem/problem-with-message",
			"title":   "Unauthorized",
			"status":  http.StatusUnauthorized,
			"detail":  "Bad credentials",
			"path":    c.Request.URL.Path,
			"message": "error.http.401",
		})
		c.Abort()
		return
	}

	jeton, err := auth.Emettre(d.Cfg.JWTSecret, d.Cfg.JWTTTL, req.Username, roles)
	if err != nil {
		d.R.Fail(c, entity.InternalError("émission du jeton impossible"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"id_token": jeton})
}

// getAuthenticate confirme l'authentification — 204 No Content.
func (d *Deps) getAuthenticate(c *gin.Context) { c.Status(http.StatusNoContent) }

// Authentifier applique les deux comportements mesurés en recette :
// jeton ABSENT → 401 avec l'enveloppe ARTP ACCES_INTERDIT (le seul code jamais émis) ;
// jeton PRÉSENT mais invalide → 401, corps vide, sans Content-Type (ANO-008).
func (d *Deps) Authentifier() gin.HandlerFunc {
	return func(c *gin.Context) {
		entete := c.GetHeader("Authorization")
		if !strings.HasPrefix(entete, "Bearer ") || strings.TrimSpace(entete[7:]) == "" {
			e := entity.AccessForbidden()
			c.JSON(http.StatusUnauthorized, httpx.Envelope{
				Success: false, Code: e.Code, Message: e.Message, Data: nil,
			})
			c.Abort()
			return
		}

		username, err := auth.Verifier(d.Cfg.JWTSecret, strings.TrimSpace(entete[7:]))
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		var ident entity.Caller
		err = d.DB.Pool.QueryRow(c,
			`SELECT u.id, u.username, o.id, o.nom
			   FROM utilisateur u JOIN operateur o ON o.id = u.operateur_id
			  WHERE u.username = $1`, username).
			Scan(&ident.UserID, &ident.Username, &ident.OperatorID, &ident.OperatorName)
		if err != nil {
			d.R.Fail(c, entity.OperatorNotFound())
			return
		}

		c.Set(cleIdentite, ident)
		c.Next()
	}
}
