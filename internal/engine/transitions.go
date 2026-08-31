package engine

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/domain"
)

// AppliquerTransition solde l'étape courante et fait passer la demande à la
// suivante. origine vaut "ACTION" (traitement nominal) ou "EXPIRATION".
func (e *Engine) AppliquerTransition(ctx context.Context, demandeID, origine string) error {
	tx, err := e.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var d domain.Demande
	var etape, statutDem, typeDem string
	err = tx.QueryRow(ctx,
		`SELECT etape_actuelle, statut_demande, type_demande,
		        operateur_source_id, operateur_destinataire_id, numero
		   FROM demande WHERE id = $1 FOR UPDATE`, demandeID).
		Scan(&etape, &statutDem, &typeDem, &d.OperateurSourceID,
			&d.OperateurDestinataireID, &d.Numero)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	if domain.StatutDemande(statutDem) != domain.StatutEnCours {
		return nil
	}

	courante := domain.Etape(etape)
	typeDemande := domain.TypeDemande(typeDem)
	maintenant := time.Now()

	statutEtapeSoldee := string(domain.EtapeTerminee)
	if origine == "EXPIRATION" {
		statutEtapeSoldee = string(domain.EtapeExpiree)
	} else if courante == domain.EtapeCompletion {
		statutEtapeSoldee = string(domain.EtapeValidee)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO etape_historique (demande_id, etape, statut, origine, date_debut, date_fin)
		 SELECT id, etape_actuelle, $2, $3, date_debut_etape, $4 FROM demande WHERE id = $1`,
		demandeID, statutEtapeSoldee, origine, maintenant); err != nil {
		return err
	}

	suivante, existe := domain.EtapeSuivante(courante)
	if !existe {
		// COMPLETION soldée : la demande se termine.
		if err := e.effetsFinDeDemande(ctx, tx, demandeID, typeDemande, d); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE demande
			    SET statut_demande = 'TERMINE', statut_etape_actuel = $2,
			        date_finalisation = $3, transition_prevue_a = NULL
			  WHERE id = $1`, demandeID, statutEtapeSoldee, maintenant); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// Effets de bord attachés à la sortie de l'étape.
	if courante == domain.EtapeActivation && typeDemande == domain.TypePortage {
		if err := e.transfererAuRegistre(ctx, tx, demandeID, d); err != nil {
			return err
		}
		if err := e.recalculerRoutage(ctx, tx, demandeID, d); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE demande
		    SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		        date_debut_etape = $3, transition_prevue_a = NULL
		  WHERE id = $1`, demandeID, string(suivante), maintenant); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// transfererAuRegistre inscrit le changement d'opérateur au registre national.
// C'est le constat central du SIT : quand une étape expire, ce transfert a lieu
// alors qu'aucun HLR n'a été touché.
func (e *Engine) transfererAuRegistre(ctx context.Context, tx pgx.Tx, demandeID string, d domain.Demande) error {
	_, err := tx.Exec(ctx,
		`UPDATE numero SET operateur_actuel_id = $2, date_dernier_portage = now()
		  WHERE msisdn IN (SELECT numero FROM demande_numero
		                    WHERE demande_id = $1 AND NOT exclu AND statut <> 'REJETE')`,
		demandeID, d.OperateurDestinataireID)
	return err
}

// recalculerRoutage finalise le routage numéro par numéro (§7.10) : préfixe du
// destinataire pour les numéros portés, de la source pour les numéros rejetés.
func (e *Engine) recalculerRoutage(ctx context.Context, tx pgx.Tx, demandeID string, d domain.Demande) error {
	var prefixeDest, prefixeSource string
	if err := tx.QueryRow(ctx, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		d.OperateurDestinataireID).Scan(&prefixeDest); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		d.OperateurSourceID).Scan(&prefixeSource); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE demande_numero
		    SET routage_info = CASE WHEN statut = 'REJETE' OR exclu THEN $2 ELSE $3 END
		  WHERE demande_id = $1`, demandeID, prefixeSource, prefixeDest); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE demande SET routage_info = $2 WHERE id = $1`,
		demandeID, prefixeDest)
	return err
}

// effetsFinDeDemande : pour une RESTITUTION ou un REVERSE, le numéro rejoint son
// opérateur d'origine et routageInfo n'apparaît qu'ici (§7.10).
func (e *Engine) effetsFinDeDemande(ctx context.Context, tx pgx.Tx, demandeID string,
	td domain.TypeDemande, d domain.Demande) error {

	if td == domain.TypePortage {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`UPDATE numero
		    SET operateur_actuel_id = $2, date_dernier_portage = now(), deja_restitue = true
		  WHERE msisdn = $1`, d.Numero, d.OperateurDestinataireID); err != nil {
		return err
	}
	var prefixe string
	if err := tx.QueryRow(ctx, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		d.OperateurDestinataireID).Scan(&prefixe); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE demande SET routage_info = $2 WHERE id = $1`, demandeID, prefixe)
	return err
}
