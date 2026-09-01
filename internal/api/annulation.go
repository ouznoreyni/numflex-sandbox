package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

func (d *Deps) routesAnnulation(g *gin.RouterGroup) {
	g.POST("/demandes/:id/annuler", d.postAnnuler)
}

// postAnnuler retire une demande avant qu'elle n'ait commencé à bouger — §7.11
// du guide. Sans corps : le seul identifiant utile est déjà dans l'URL.
//
// [HYP] verifierGel n'est pas appelé ici, contrairement aux trois endpoints de
// traitement (acceptation, a-confirmer, traitement). Le gel bloque le
// *traitement* des demandes pendant un incident interne (BR-012) ; annuler
// une demande encore à ACCEPTATION ne fait avancer aucune étape, c'est un
// retrait, pas un traitement. Le guide ne tranche pas explicitement et rien
// ne le mesure au SIT.
func (d *Deps) postAnnuler(c *gin.Context) {
	id := c.Param("id")

	dm, errCh := d.chargerDemande(c, id)
	if errCh != nil {
		d.R.Fail(c, errCh)
		return
	}
	if err := entity.CanCancel(dm, Appelant(c).OperatorID); err != nil {
		d.R.Fail(c, err)
		return
	}

	tx, errTx := d.DB.Pool.Begin(c)
	if errTx != nil {
		d.R.Fail(c, entity.InternalError("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	maintenant := time.Now()
	if _, err := tx.Exec(c,
		`INSERT INTO etape_historique
		   (demande_id, etape, statut, operateur_id, origine, commentaire, date_debut, date_fin)
		 SELECT id, etape_actuelle, 'TERMINE', $2, 'ACTION', NULL, date_debut_etape, $3
		   FROM demande WHERE id = $1`,
		dm.ID, Appelant(c).OperatorID, maintenant); err != nil {
		d.R.Fail(c, entity.InternalError("annulation de la demande"))
		return
	}

	if _, err := tx.Exec(c,
		`UPDATE demande
		    SET statut_demande = 'ANNULE', statut_etape_actuel = 'TERMINE',
		        date_finalisation = $2, transition_prevue_a = NULL
		  WHERE id = $1`,
		dm.ID, maintenant); err != nil {
		d.R.Fail(c, entity.InternalError("annulation de la demande"))
		return
	}

	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, entity.InternalError("validation de la transaction"))
		return
	}

	dto, err := d.demandeDTO(c, dm.ID)
	if err != nil {
		d.R.Fail(c, entity.InternalError("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusOK, "Demande annulée avec succès", dto)
}
