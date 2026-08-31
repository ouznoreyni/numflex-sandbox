package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
)

func (d *Deps) routesTraitement(g *gin.RouterGroup) {
	g.POST("/demandes/traitement", d.postTraitement)
}

// reqTraitement ne déclare PAS de champ etape : retiré en v2. Un client v1 qui
// l'envoie encore n'est ni rejeté ni averti — le champ est simplement ignoré
// et l'étape courante est exécutée (ANO-018).
type reqTraitement struct {
	IDDemande   string `json:"idDemande"`
	Commentaire string `json:"commentaire"`
}

func (d *Deps) postTraitement(c *gin.Context) {
	if d.verifierGel(c) {
		return
	}

	var req reqTraitement
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if req.IDDemande == "" {
		d.R.Fail(c, apperr.Validation(apperr.FieldError{
			ObjectName: "traitementDTO", Field: "idDemande", Message: "ne doit pas être vide",
		}))
		return
	}

	dm, e := d.chargerDemande(c, req.IDDemande)
	if e != nil {
		d.R.Fail(c, e)
		return
	}
	if e := domain.PeutTraiter(dm, Appelant(c).OperateurID); e != nil {
		d.R.Fail(c, e)
		return
	}

	// ANO-005 : la COMPLETION est la seule étape lente — ~30 s mesurés.
	if dm.EtapeActuelle == domain.EtapeCompletion && d.Cfg.CompletionLatency > 0 {
		time.Sleep(d.Cfg.CompletionLatency)
	}

	if req.Commentaire != "" {
		if _, err := d.DB.Pool.Exec(c,
			`UPDATE demande SET commentaire = $2 WHERE id = $1`,
			dm.ID, req.Commentaire); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("enregistrement du commentaire"))
			return
		}
	}
	if err := d.Moteur.PlanifierTransition(c, dm.ID); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("planification de la transition"))
		return
	}

	// La demande est relue AVANT que la transition ne soit appliquée : la réponse
	// porte donc l'étape précédente. C'est le comportement mesuré (R-10) — un
	// client qui enchaîne sur ce corps émet l'étape suivante trop tôt.
	dto, err := d.demandeDTO(c, dm.ID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusOK, "Étape traitée avec succès", dto)
}
