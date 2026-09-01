package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
)

func (d *Deps) routesAcceptation(g *gin.RouterGroup) {
	g.POST("/demandes/acceptation", d.postAcceptation)
	g.POST("/demandes/:id/acceptation", d.postAcceptationFlotte)
}

// verifierGel refuse tout traitement pendant qu'un incident interne fige la
// place — BR-012, §6.5 du guide. Le même appel ouvre /a-confirmer (Task 15) et
// /traitement (Task 16).
func (d *Deps) verifierGel(c *gin.Context) bool {
	if gelee, _ := d.Moteur.PlaceGelee(c); gelee {
		d.R.Fail(c, apperr.ErreurInterne(
			"Le traitement des demandes est gelé par un incident interne en cours."))
		return true
	}
	return false
}

// --- Particulier / restitution ----------------------------------------------

type reqAcceptation struct {
	IDDemande    string `json:"idDemande"`
	Accepte      bool   `json:"accepte"`
	MotifRejetID string `json:"motifRejetId"`
	Commentaire  string `json:"commentaire"`
}

func (d *Deps) postAcceptation(c *gin.Context) {
	if d.verifierGel(c) {
		return
	}

	var req reqAcceptation
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	// Rupture v1 → v2 : la demande n'est plus identifiée par numero (R-10 §8).
	// Un corps qui n'envoie que numero laisse idDemande vide et échoue ici.
	if req.IDDemande == "" {
		d.R.Fail(c, apperr.Validation(apperr.FieldError{
			ObjectName: "acceptationDTO", Field: "idDemande",
			Message: "ne doit pas être vide",
		}))
		return
	}

	dm, errCh := d.chargerDemande(c, req.IDDemande)
	if errCh != nil {
		d.R.Fail(c, errCh)
		return
	}
	if err := domain.PeutAccepter(dm, Appelant(c).OperateurID); err != nil {
		d.R.Fail(c, err)
		return
	}
	// Le brief l'énonce sans condition : un motifRejetId renseigné doit exister,
	// que la demande soit acceptée ou rejetée. Un identifiant valide envoyé
	// inutilement avec accepte:true continue de passer — seul un identifiant
	// inconnu est refusé.
	if req.MotifRejetID != "" {
		existe, err := d.motifExiste(c, req.MotifRejetID)
		if err != nil {
			d.R.Fail(c, apperr.ErreurInterne("vérification du motif de rejet"))
			return
		}
		if !existe {
			d.R.Fail(c, apperr.ValidationEchouee("Motif de rejet inconnu"))
			return
		}
	}

	if !req.Accepte {
		d.traiterRejetDemande(c, dm.ID, req.MotifRejetID, req.Commentaire)
		return
	}
	d.traiterAcceptationDemande(c, dm.ID, req.Commentaire)
}

// --- Entreprise / flotte -----------------------------------------------------

type numeroRejeteFlotte struct {
	Numero       string `json:"numero"`
	MotifRejetID string `json:"motifRejetId"`
}

type reqAcceptationFlotte struct {
	Accepte        bool                 `json:"accepte"`
	NumerosRejetes []numeroRejeteFlotte `json:"numerosRejetes"`
	MotifRejetID   string               `json:"motifRejetId"`
	Commentaire    string               `json:"commentaire"`
}

func (d *Deps) postAcceptationFlotte(c *gin.Context) {
	if d.verifierGel(c) {
		return
	}
	id := c.Param("id")

	var req reqAcceptationFlotte
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}

	dm, errCh := d.chargerDemande(c, id)
	if errCh != nil {
		d.R.Fail(c, errCh)
		return
	}
	if err := domain.PeutAccepter(dm, Appelant(c).OperateurID); err != nil {
		d.R.Fail(c, err)
		return
	}
	// Même contrôle inconditionnel que le particulier : un motifRejetId de
	// premier niveau renseigné doit exister, avant l'embranchement.
	if req.MotifRejetID != "" {
		existe, err := d.motifExiste(c, req.MotifRejetID)
		if err != nil {
			d.R.Fail(c, apperr.ErreurInterne("vérification du motif de rejet"))
			return
		}
		if !existe {
			d.R.Fail(c, apperr.ValidationEchouee("Motif de rejet inconnu"))
			return
		}
	}

	// Rejet total : même traitement qu'un particulier, la flotte entière tombe.
	if !req.Accepte {
		d.traiterRejetDemande(c, dm.ID, req.MotifRejetID, req.Commentaire)
		return
	}

	// Rejet partiel : chaque numéro visé doit appartenir à la flotte, et son
	// motif — s'il en porte un — doit exister. On valide tout avant de rien
	// écrire, pour ne jamais laisser une flotte à moitié marquée.
	for _, nr := range req.NumerosRejetes {
		var appartient bool
		if err := d.DB.Pool.QueryRow(c,
			`SELECT EXISTS (SELECT 1 FROM demande_numero WHERE demande_id = $1 AND numero = $2)`,
			dm.ID, nr.Numero).Scan(&appartient); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("vérification du numéro"))
			return
		}
		if !appartient {
			d.R.Fail(c, apperr.ValidationEchouee(
				fmt.Sprintf("Le numéro %s ne fait pas partie de cette demande", nr.Numero)))
			return
		}
		if nr.MotifRejetID != "" {
			existe, err := d.motifExiste(c, nr.MotifRejetID)
			if err != nil {
				d.R.Fail(c, apperr.ErreurInterne("vérification du motif de rejet"))
				return
			}
			if !existe {
				d.R.Fail(c, apperr.ValidationEchouee("Motif de rejet inconnu"))
				return
			}
		}
	}

	tx, errTx := d.DB.Pool.Begin(c)
	if errTx != nil {
		d.R.Fail(c, apperr.ErreurInterne("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	for _, nr := range req.NumerosRejetes {
		if _, err := tx.Exec(c,
			`UPDATE demande_numero SET statut = 'REJETE', motif_rejet_id = NULLIF($3, '')
			  WHERE demande_id = $1 AND numero = $2`,
			dm.ID, nr.Numero, nr.MotifRejetID); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("rejet du numéro"))
			return
		}
	}

	// [HYP] Le guide ne dit nulle part ce qu'il advient d'une flotte rejetée
	// numéro par numéro jusqu'à épuisement complet ; ni mesuré au SIT, ni fixé
	// par un test. Le projet a choisi que la demande n'a alors plus rien à
	// porter et bascule REJETE elle aussi, sans transition à planifier.
	var resteEligible bool
	if err := tx.QueryRow(c,
		`SELECT EXISTS (SELECT 1 FROM demande_numero WHERE demande_id = $1 AND statut <> 'REJETE')`,
		dm.ID).Scan(&resteEligible); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("vérification de la flotte"))
		return
	}
	if !resteEligible {
		if err := d.rejeterDemande(c, tx, dm.ID, Appelant(c).OperateurID, "",
			req.Commentaire, time.Now()); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("rejet de la demande"))
			return
		}
		if err := tx.Commit(c); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("validation de la transaction"))
			return
		}
		d.repondreDemandeTraitee(c, dm.ID)
		return
	}

	if _, err := tx.Exec(c, `UPDATE demande SET commentaire = NULLIF($2, '') WHERE id = $1`,
		dm.ID, req.Commentaire); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement du commentaire"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("validation de la transaction"))
		return
	}

	if err := d.Moteur.PlanifierTransition(c, dm.ID); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("planification de la transition"))
		return
	}
	d.repondreDemandeTraitee(c, dm.ID)
}

// --- Partagé -----------------------------------------------------------------

// motifExiste vérifie qu'un motifRejetId renseigné correspond bien à une ligne
// du référentiel motif_rejet.
func (d *Deps) motifExiste(c *gin.Context, motifID string) (bool, error) {
	var existe bool
	err := d.DB.Pool.QueryRow(c,
		`SELECT EXISTS (SELECT 1 FROM motif_rejet WHERE id = $1)`, motifID).Scan(&existe)
	return existe, err
}

// traiterRejetDemande solde définitivement une demande : REJETE, étape TERMINE,
// motif_rejet_id enregistré, et une ligne etape_historique d'origine ACTION —
// car aucune transition du moteur ne viendra jamais l'écrire (R-10 ne s'applique
// qu'à l'acceptation).
func (d *Deps) traiterRejetDemande(c *gin.Context, id, motifRejetID, commentaire string) {
	if motifRejetID == "" {
		d.R.Fail(c, apperr.MotifRejetObligatoire())
		return
	}
	existe, err := d.motifExiste(c, motifRejetID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("vérification du motif de rejet"))
		return
	}
	if !existe {
		d.R.Fail(c, apperr.ValidationEchouee("Motif de rejet inconnu"))
		return
	}

	tx, errTx := d.DB.Pool.Begin(c)
	if errTx != nil {
		d.R.Fail(c, apperr.ErreurInterne("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	if err := d.rejeterDemande(c, tx, id, Appelant(c).OperateurID, motifRejetID,
		commentaire, time.Now()); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("rejet de la demande"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("validation de la transaction"))
		return
	}
	d.repondreDemandeTraitee(c, id)
}

// traiterAcceptationDemande enregistre le commentaire puis planifie la
// transition — sans écrire elle-même de ligne etape_historique : c'est le
// moteur qui la posera au moment de la convergence (Task 9).
func (d *Deps) traiterAcceptationDemande(c *gin.Context, id, commentaire string) {
	if _, err := d.DB.Pool.Exec(c,
		`UPDATE demande SET commentaire = NULLIF($2, '') WHERE id = $1`, id, commentaire); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement du commentaire"))
		return
	}
	if err := d.Moteur.PlanifierTransition(c, id); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("planification de la transition"))
		return
	}
	d.repondreDemandeTraitee(c, id)
}

// rejeterDemande porte l'écriture commune à un rejet individuel et à un rejet
// de flotte devenu total : la ligne etape_historique puis la clôture de la
// demande, dans la même transaction que l'appelant a ouverte.
func (d *Deps) rejeterDemande(c *gin.Context, tx pgx.Tx, id, operateurID, motifRejetID,
	commentaire string, maintenant time.Time) error {

	if _, err := tx.Exec(c,
		`INSERT INTO etape_historique
		   (demande_id, etape, statut, operateur_id, origine, commentaire, date_debut, date_fin)
		 SELECT id, etape_actuelle, 'TERMINE', $2, 'ACTION', NULLIF($3, ''), date_debut_etape, $4
		   FROM demande WHERE id = $1`,
		id, operateurID, commentaire, maintenant); err != nil {
		return err
	}

	_, err := tx.Exec(c,
		`UPDATE demande
		    SET statut_demande = 'REJETE', statut_etape_actuel = 'TERMINE',
		        date_finalisation = $2, motif_rejet_id = NULLIF($3, ''), commentaire = NULLIF($4, '')
		  WHERE id = $1`,
		id, maintenant, motifRejetID, commentaire)
	return err
}

// repondreDemandeTraitee rend la réponse commune aux deux handlers : le message
// exact du contrat, avec la demande relue après l'écriture. Sur acceptation, la
// transition n'est pas encore appliquée — le DTO porte donc encore ACCEPTATION,
// c'est le comportement mesuré (R-10), pas un défaut.
func (d *Deps) repondreDemandeTraitee(c *gin.Context, id string) {
	dto, err := d.demandeDTO(c, id)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusOK, "Décision d'acceptation enregistrée", dto)
}
