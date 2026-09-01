package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
)

func (d *Deps) routesConfirmation(g *gin.RouterGroup) {
	g.POST("/demandes/a-confirmer", d.postAConfirmer)
}

type reqConfirmation struct {
	IDDemande   string `json:"idDemande"`
	Commentaire string `json:"commentaire"`
}

// postAConfirmer traite l'étape CONFIRMATION : mesuré au SIT, un PORTAGE se
// solde quand tous les opérateurs de la place SAUF le destinataire ont
// confirmé (celui-ci est auto-confirmé) ; une RESTITUTION ou un REVERSE exige
// tout le monde, destinataire compris. La décision d'appartenance passe
// exclusivement par domain.ConfirmateursAttendus — la même fonction que la
// file /a-confirmer (Task 13) — pour que les deux ne puissent jamais diverger.
func (d *Deps) postAConfirmer(c *gin.Context) {
	if d.verifierGel(c) {
		return
	}

	var req reqConfirmation
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if req.IDDemande == "" {
		d.R.Fail(c, apperr.Validation(apperr.FieldError{
			ObjectName: "confirmationDTO", Field: "idDemande",
			Message: "ne doit pas être vide",
		}))
		return
	}

	dm, errCh := d.chargerDemande(c, req.IDDemande)
	if errCh != nil {
		d.R.Fail(c, errCh)
		return
	}
	if dm.StatutDemande != domain.StatutEnCours || dm.EtapeActuelle != domain.EtapeConfirmation {
		d.R.Fail(c, apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est pas à l'étape CONFIRMATION (étape actuelle : %s).", dm.EtapeActuelle)))
		return
	}
	if dm.TransitionEnAttente {
		d.R.Fail(c, apperr.EtapeInvalide(
			"L'étape CONFIRMATION a déjà été soldée pour cette demande."))
		return
	}

	tousOps, errOps := d.tousOperateurs(c)
	if errOps != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des opérateurs"))
		return
	}
	appelantID := Appelant(c).OperateurID
	attendu := false
	for _, op := range domain.ConfirmateursAttendus(dm, tousOps) {
		if op == appelantID {
			attendu = true
			break
		}
	}
	if !attendu {
		d.R.Fail(c, apperr.DemandeAccesRefuse(
			"Votre opérateur n'a pas à confirmer cette demande."))
		return
	}

	// Anti-rejeu : la garantie vient de la clé primaire (demande_id, operateur_id)
	// de la table confirmation, pas d'une lecture préalable — un pré-contrôle
	// serait sujet à une course entre deux appels concurrents du même opérateur.
	_, err := d.DB.Pool.Exec(c,
		`INSERT INTO confirmation (demande_id, operateur_id, commentaire, date_conf)
		 VALUES ($1, $2, NULLIF($3, ''), $4)`,
		dm.ID, appelantID, req.Commentaire, time.Now())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			d.R.Fail(c, apperr.DemandeAccesRefuse(
				"Votre opérateur a déjà confirmé cette demande."))
			return
		}
		d.R.Fail(c, apperr.ErreurInterne("enregistrement de la confirmation"))
		return
	}

	var nbConfirmations int
	if err := d.DB.Pool.QueryRow(c,
		`SELECT COUNT(*) FROM confirmation WHERE demande_id = $1`, dm.ID).
		Scan(&nbConfirmations); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("comptage des confirmations"))
		return
	}
	if nbConfirmations >= len(domain.ConfirmateursAttendus(dm, tousOps)) {
		if err := d.Moteur.PlanifierTransition(c, dm.ID); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("planification de la transition"))
			return
		}
	}

	dto, errDTO := d.demandeDTO(c, dm.ID)
	if errDTO != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande"))
		return
	}
	// Sans client : les captures des deux confirmations, orange puis expresso,
	// n'en portent pas, là où toutes les autres réponses en portent un.
	d.R.OK(c, http.StatusOK, "Étape traitée avec succès", sansClient(dto))
}
