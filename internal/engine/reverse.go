package engine

import (
	"context"
	"time"

	"github.com/yas/numflex-sandbox/internal/oid"
	"github.com/yas/numflex-sandbox/internal/store"
)

// ValiderReverse est un acte de l'ARTP, hors périmètre de l'API gateway (§6).
// Il crée une Demande de type REVERSE directement à l'étape CONFIRMATION : ni
// ACCEPTATION, ni DESACTIVATION/ACTIVATION.
func ValiderReverse(ctx context.Context, db *store.DB, reverseID string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var numero, operateurOrigine, statut string
	if err := tx.QueryRow(ctx,
		`SELECT numero, operateur_id, statut FROM reverse_request WHERE id = $1 FOR UPDATE`,
		reverseID).Scan(&numero, &operateurOrigine, &statut); err != nil {
		return err
	}
	if statut != "EN_ATTENTE" {
		return nil
	}

	var detenteurActuel string
	if err := tx.QueryRow(ctx,
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = $1`, numero).
		Scan(&detenteurActuel); err != nil {
		return err
	}

	id := oid.New()
	maintenant := time.Now()
	if _, err := tx.Exec(ctx,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, date_demande, date_debut_etape)
		 VALUES ($1,$2,'PARTICULIER','REVERSE','EN_COURS','CONFIRMATION','EN_COURS',
		         $3,$4,$4,$5,$5)`,
		id, numero, detenteurActuel, operateurOrigine, maintenant); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO demande_numero (demande_id, numero, statut) VALUES ($1,$2,'EN_COURS')`,
		id, numero); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE reverse_request SET statut='VALIDE', date_decision=$2, demande_id=$3
		  WHERE id = $1`, reverseID, maintenant, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RejeterReverse est également un acte de l'ARTP : rejeter la demande sans
// jamais créer de Demande.
func RejeterReverse(ctx context.Context, db *store.DB, reverseID string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE reverse_request SET statut='REJETE', date_decision=now()
		  WHERE id = $1 AND statut = 'EN_ATTENTE'`, reverseID)
	return err
}

// validerReversesAutomatiquement rejoue ValiderReverse pour toute demande
// EN_ATTENTE depuis plus de REVERSE_AUTO_VALIDATION_SECONDS. Désactivé par
// défaut (0 = jamais) : dans le monde réel, la validation est un acte humain
// de l'ARTP, hors API ; ce délai n'existe que pour permettre au sandbox de
// simuler l'aval du régulateur sans intervention du CLI.
func (e *Engine) validerReversesAutomatiquement(ctx context.Context) error {
	if e.cfg.ReverseAutoValidation <= 0 {
		return nil
	}
	rows, err := e.db.Pool.Query(ctx,
		`SELECT id FROM reverse_request
		  WHERE statut = 'EN_ATTENTE' AND date_demande + make_interval(secs => $1) <= now()`,
		e.cfg.ReverseAutoValidation.Seconds())
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range ids {
		if err := ValiderReverse(ctx, e.db, id); err != nil {
			return err
		}
	}
	return nil
}

// completerReversesConfirmes : la COMPLETION d'un REVERSE est réservée à l'ARTP.
// Aucun endpoint ne l'expose ; c'est le moteur qui la prononce une fois que tous
// les opérateurs ont confirmé.
func (e *Engine) completerReversesConfirmes(ctx context.Context) error {
	rows, err := e.db.Pool.Query(ctx,
		`SELECT d.id FROM demande d
		  WHERE d.type_demande = 'REVERSE'
		    AND d.statut_demande = 'EN_COURS'
		    AND d.etape_actuelle = 'CONFIRMATION'
		    AND d.transition_prevue_a IS NULL
		    AND (SELECT count(*) FROM confirmation c WHERE c.demande_id = d.id)
		        >= (SELECT count(*) FROM operateur)`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range ids {
		// CONFIRMATION → COMPLETION, puis COMPLETION → TERMINE.
		if err := e.AppliquerTransition(ctx, id, "ACTION"); err != nil {
			return err
		}
		if err := e.AppliquerTransition(ctx, id, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}
