package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/domain"
)

func (d *Deps) routesLecture(g *gin.RouterGroup) {
	g.GET("/demandes/mes-demandes", d.getMesDemandes)
	g.GET("/demandes/a-accepter", d.getAAccepter)
	g.GET("/demandes/a-accepter/:id", d.getAAccepterDetail)
	g.GET("/demandes/a-traiter", d.getATraiter)
	g.GET("/demandes/a-traiter/:id", d.getATraiterDetail)
	g.GET("/demandes/a-confirmer", d.getAConfirmer)
	g.GET("/demandes/a-confirmer/:id", d.getAConfirmerDetail)
	g.GET("/demandes/deja-confirmees", d.getDejaConfirmees)
	g.GET("/demandes/in", d.getIn)
	g.GET("/demandes/out", d.getOut)
}

// idsDemandes exécute une requête dont la seule colonne est l'id de demande,
// pour les listes dont le filtre se réduit à une clause WHERE sur la table
// demande. Le tri par date_demande reproduit l'ordre chronologique attendu
// d'une file de travail.
func (d *Deps) idsDemandes(ctx context.Context, filtre string, args ...any) ([]string, error) {
	rows, err := d.DB.Pool.Query(ctx,
		`SELECT id FROM demande dm WHERE `+filtre+` ORDER BY date_demande`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// rendreListe convertit une liste d'ids en DTOs via demandeDTO et rend la
// réponse. La liste vide s'initialise en dehors de cette fonction pour que
// l'appelant ne perde jamais le [] : demandeDTO peut échouer avant le premier
// append.
func (d *Deps) rendreListe(c *gin.Context, message string, ids []string) {
	out := []map[string]any{}
	for _, id := range ids {
		dto, err := d.demandeDTO(c, id)
		if err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture de la demande"))
			return
		}
		out = append(out, dto)
	}
	d.R.OK(c, http.StatusOK, message, out)
}

// --- mes-demandes ------------------------------------------------------------

func (d *Deps) getMesDemandes(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsDemandes(c,
		`dm.operateur_source_id = $1 OR dm.operateur_destinataire_id = $1`,
		appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes"))
		return
	}
	d.rendreListe(c, "Demandes récupérées avec succès", ids)
}

// --- a-accepter ---------------------------------------------------------------

func filtreAAccepter() string {
	return `dm.statut_demande = 'EN_COURS' AND dm.etape_actuelle = 'ACCEPTATION'
	         AND dm.operateur_source_id = $1`
}

func (d *Deps) getAAccepter(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsDemandes(c, filtreAAccepter(), appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes à accepter"))
		return
	}
	d.rendreListe(c, "Demandes à accepter récupérées avec succès", ids)
}

func (d *Deps) getAAccepterDetail(c *gin.Context) {
	d.detailFiltre(c, filtreAAccepter(), c.Param("id"), Appelant(c).OperateurID)
}

// --- a-traiter ------------------------------------------------------------

// filtreATraiter exprime en SQL domain.ResponsableEtape : DESACTIVATION incombe
// à la source, ACTIVATION et COMPLETION au destinataire, sauf la COMPLETION
// d'un REVERSE qui revient à l'ARTP (aucun opérateur ne la voit ici).
func filtreATraiter() string {
	return `dm.statut_demande = 'EN_COURS' AND (
	           (dm.etape_actuelle = 'DESACTIVATION' AND dm.operateur_source_id = $1)
	        OR (dm.etape_actuelle IN ('ACTIVATION', 'COMPLETION')
	            AND dm.operateur_destinataire_id = $1
	            AND NOT (dm.etape_actuelle = 'COMPLETION' AND dm.type_demande = 'REVERSE'))
	        )`
}

func (d *Deps) getATraiter(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsDemandes(c, filtreATraiter(), appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes à traiter"))
		return
	}
	d.rendreListe(c, "Demandes à traiter récupérées avec succès", ids)
}

func (d *Deps) getATraiterDetail(c *gin.Context) {
	d.detailFiltre(c, filtreATraiter(), c.Param("id"), Appelant(c).OperateurID)
}

// --- a-confirmer ------------------------------------------------------------
//
// Le filtre ne se réduit pas à une clause WHERE sur demande : l'appartenance à
// ConfirmateursAttendus dépend du type de demande et n'est pas exprimable par
// une seule comparaison de colonnes (elle exclut le destinataire seulement pour
// un PORTAGE). On charge donc les candidats EN_COURS/CONFIRMATION puis on filtre
// en Go avec la règle de domain.ConfirmateursAttendus, pour ne jamais dupliquer
// cette règle en SQL.

func (d *Deps) getAConfirmer(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsAConfirmer(c, appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes à confirmer"))
		return
	}
	d.rendreListe(c, "Demandes à confirmer récupérées avec succès", ids)
}

func (d *Deps) getAConfirmerDetail(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsAConfirmer(c, appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes à confirmer"))
		return
	}
	d.detailParmi(c, ids, c.Param("id"))
}

// idsAConfirmer liste les demandes EN_COURS/CONFIRMATION où l'opérateur donné
// est un confirmateur attendu (domain.ConfirmateursAttendus) et n'a pas encore
// confirmé.
func (d *Deps) idsAConfirmer(ctx context.Context, operateurID string) ([]string, error) {
	// Chargé avant d'ouvrir le curseur des candidats, pour ne jamais tenir deux
	// requêtes en cours sur le pool en même temps.
	tousOps, errOps := d.tousOperateurs(ctx)
	if errOps != nil {
		return nil, errOps
	}

	rows, err := d.DB.Pool.Query(ctx, `
		SELECT dm.id, dm.type_demande, dm.operateur_destinataire_id,
		       EXISTS (SELECT 1 FROM confirmation c
		                WHERE c.demande_id = dm.id AND c.operateur_id = $1)
		  FROM demande dm
		 WHERE dm.statut_demande = 'EN_COURS' AND dm.etape_actuelle = 'CONFIRMATION'
		 ORDER BY dm.date_demande`, operateurID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id, typeDem, destinataireID string
		var dejaConfirme bool
		if err := rows.Scan(&id, &typeDem, &destinataireID, &dejaConfirme); err != nil {
			return nil, err
		}
		if dejaConfirme {
			continue
		}
		dm := domain.Demande{
			TypeDemande:             domain.TypeDemande(typeDem),
			OperateurDestinataireID: destinataireID,
		}
		attendu := false
		for _, op := range domain.ConfirmateursAttendus(dm, tousOps) {
			if op == operateurID {
				attendu = true
				break
			}
		}
		if attendu {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// --- deja-confirmees --------------------------------------------------------

func (d *Deps) getDejaConfirmees(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsDejaConfirmees(c, appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes déjà confirmées"))
		return
	}
	d.rendreListe(c, "Demandes déjà confirmées récupérées avec succès", ids)
}

// idsDejaConfirmees liste les demandes que l'opérateur a confirmées.
//
// ANO-019 — c'est le seul endroit du projet où une requête SQL dépend du mode
// de fidélité (d.R.Fidelity()). En mode real, la plateforme mesurée omet de
// cette liste les confirmations émises par l'opérateur source de la demande :
// ORANGE, source d'un portage, confirme avec succès mais sa propre
// confirmation n'apparaît jamais dans /deja-confirmees ; celle d'un tiers si.
// En mode contract ce filtre n'existe pas.
func (d *Deps) idsDejaConfirmees(ctx context.Context, operateurID string) ([]string, error) {
	filtre := `c.operateur_id = $1`
	if d.R.Fidelity() == config.FidelityReal {
		filtre += ` AND c.operateur_id <> dm.operateur_source_id`
	}
	rows, err := d.DB.Pool.Query(ctx, `
		SELECT dm.id FROM demande dm
		  JOIN confirmation c ON c.demande_id = dm.id
		 WHERE `+filtre+`
		 ORDER BY dm.date_demande`, operateurID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// --- in / out -----------------------------------------------------------------

func (d *Deps) getIn(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsDemandes(c,
		`dm.type_demande = 'PORTAGE' AND dm.statut_demande = 'TERMINE'
		  AND dm.operateur_destinataire_id = $1`, appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes IN"))
		return
	}
	d.rendreListe(c, "Demandes IN récupérées avec succès", ids)
}

func (d *Deps) getOut(c *gin.Context) {
	appelant := Appelant(c)
	ids, err := d.idsDemandes(c,
		`dm.type_demande = 'PORTAGE' AND dm.statut_demande = 'TERMINE'
		  AND dm.operateur_source_id = $1`, appelant.OperateurID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des demandes OUT"))
		return
	}
	d.rendreListe(c, "Demandes OUT récupérées avec succès", ids)
}

// --- détails partagés --------------------------------------------------------

// detailFiltre réutilise le filtre d'une liste réduite à une clause WHERE sur
// demande : une demande qui ne le satisfait pas n'existe pas pour cet endpoint.
func (d *Deps) detailFiltre(c *gin.Context, filtre, id, operateurID string) {
	var trouve bool
	err := d.DB.Pool.QueryRow(c,
		`SELECT EXISTS (SELECT 1 FROM demande dm WHERE dm.id = $2 AND `+filtre+`)`,
		operateurID, id).Scan(&trouve)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture de la demande"))
		return
	}
	if !trouve {
		d.R.Fail(c, apperr.DemandeNonTrouvee())
		return
	}
	d.detailUnique(c, id)
}

// detailParmi réutilise une liste d'ids déjà calculée (a-confirmer, dont le
// filtre n'est pas une simple clause SQL).
func (d *Deps) detailParmi(c *gin.Context, ids []string, id string) {
	for _, x := range ids {
		if x == id {
			d.detailUnique(c, id)
			return
		}
	}
	d.R.Fail(c, apperr.DemandeNonTrouvee())
}

func (d *Deps) detailUnique(c *gin.Context, id string) {
	dto, err := d.demandeDTO(c, id)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture de la demande"))
		return
	}
	// [HYP] Le guide ne documente pas le message de succès d'un détail unitaire ;
	// ni la brief ni les tests ne le fixent. Choisi par cohérence avec les
	// messages de liste ("... récupérée(s) avec succès").
	d.R.OK(c, http.StatusOK, "Demande récupérée avec succès", dto)
}

// --- helpers exposés aux tâches suivantes ------------------------------------

func (d *Deps) chargerDemande(ctx context.Context, id string) (domain.Demande, *apperr.Error) {
	var dm domain.Demande
	var etape, statutDem, statutEtape, typeDem, typeAb string
	var transition *string

	err := d.DB.Pool.QueryRow(ctx,
		`SELECT id, numero, type_demande, type_abonne, statut_demande, etape_actuelle,
		        statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		        createur_operateur_id, transition_prevue_a::text
		   FROM demande WHERE id = $1`, id).
		Scan(&dm.ID, &dm.Numero, &typeDem, &typeAb, &statutDem, &etape, &statutEtape,
			&dm.OperateurSourceID, &dm.OperateurDestinataireID,
			&dm.CreateurOperateurID, &transition)
	if errors.Is(err, pgx.ErrNoRows) {
		return dm, apperr.DemandeNonTrouvee()
	}
	if err != nil {
		return dm, apperr.ErreurInterne("lecture de la demande")
	}

	dm.TypeDemande = domain.TypeDemande(typeDem)
	dm.TypeAbonne = domain.TypeAbonne(typeAb)
	dm.StatutDemande = domain.StatutDemande(statutDem)
	dm.EtapeActuelle = domain.Etape(etape)
	dm.StatutEtapeActuel = domain.StatutEtape(statutEtape)
	dm.TransitionEnAttente = transition != nil
	return dm, nil
}

func (d *Deps) tousOperateurs(ctx context.Context) ([]string, error) {
	rows, err := d.DB.Pool.Query(ctx, `SELECT id FROM operateur ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
