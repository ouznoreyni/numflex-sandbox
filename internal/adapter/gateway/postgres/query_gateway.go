package postgres

import (
	"context"
	"fmt"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// QueryGateway is the Postgres implementation of port.QueryGateway. Its
// methods carry the SQL that used to live in
// internal/api/demandes_lecture.go, unchanged in substance: the same six
// SELECT statements (idsDemandes, its a-confirmer and déjà-confirmées
// variants, tousOperateurs, and the a-accepter/a-traiter detail EXISTS
// query), now behind one method per queue instead of one *Deps method per
// route. The a-confirmer filtering that cannot be expressed as a single SQL
// predicate — entity.ExpectedConfirmers, since a PORTAGE excludes its
// recipient but a RESTITUTION/REVERSE does not — still runs in Go, exactly
// as it did in the legacy idsAConfirmer: entity is the innermost layer, so
// an adapter calling into it is not a layering violation.
type QueryGateway struct {
	db Querier
}

// NewQueryGateway returns a gateway bound to db — always the plain pool in
// practice: none of the seven read-only queues ever runs inside a
// transaction.
func NewQueryGateway(db Querier) *QueryGateway {
	return &QueryGateway{db: db}
}

// ids exécute une requête dont la seule colonne est l'id de demande, pour
// les listes dont le filtre se réduit à une clause WHERE sur la table
// demande. Le tri par date_demande reproduit l'ordre chronologique attendu
// d'une file de travail — moved verbatim from Deps.idsDemandes.
func (g *QueryGateway) ids(ctx context.Context, filtre string, args ...any) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM demande dm WHERE `+filtre+` ORDER BY date_demande`, args...)
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

func (g *QueryGateway) Own(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx,
		`dm.operateur_source_id = $1 OR dm.operateur_destinataire_id = $1`, operatorID)
}

// filtreAAccepter — moved verbatim from Deps.filtreAAccepter.
func filtreAAccepter() string {
	return `dm.statut_demande = 'EN_COURS' AND dm.etape_actuelle = 'ACCEPTATION'
	         AND dm.operateur_source_id = $1`
}

func (g *QueryGateway) ToAccept(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx, filtreAAccepter(), operatorID)
}

// filtreATraiter — moved verbatim from Deps.filtreATraiter, comment
// included: ACCEPTATION y figure : la capture « 1.orange1_EN_COURS_Demandes
// à traiter_next_ACCEPTATION » montre une demande à cette étape dans la
// file de la source. a-traiter répond à « nécessitant une action de votre
// part » (§7.7), pas à un sous-ensemble d'étapes — a-accepter n'en est
// qu'une vue spécialisée.
func filtreATraiter() string {
	return `dm.statut_demande = 'EN_COURS' AND (
	           (dm.etape_actuelle IN ('ACCEPTATION', 'DESACTIVATION')
	            AND dm.operateur_source_id = $1)
	        OR (dm.etape_actuelle IN ('ACTIVATION', 'COMPLETION')
	            AND dm.operateur_destinataire_id = $1
	            AND NOT (dm.etape_actuelle = 'COMPLETION' AND dm.type_demande = 'REVERSE'))
	        )`
}

func (g *QueryGateway) ToProcess(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx, filtreATraiter(), operatorID)
}

func (g *QueryGateway) Incoming(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx,
		`dm.type_demande = 'PORTAGE' AND dm.statut_demande = 'TERMINE'
		  AND dm.operateur_destinataire_id = $1`, operatorID)
}

func (g *QueryGateway) Outgoing(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx,
		`dm.type_demande = 'PORTAGE' AND dm.statut_demande = 'TERMINE'
		  AND dm.operateur_source_id = $1`, operatorID)
}

// AlreadyConfirmed — moved verbatim from Deps.idsDejaConfirmees.
//
// ANO-019 — c'est le seul endroit du projet où une requête SQL dépend du
// mode de fidélité. En mode real, la plateforme mesurée omet de cette liste
// les confirmations émises par l'opérateur source de la demande : ORANGE,
// source d'un portage, confirme avec succès mais sa propre confirmation
// n'apparaît jamais dans /deja-confirmees ; celle d'un tiers si. En mode
// contract ce filtre n'existe pas — excludeSource porte ce choix depuis la
// couche de câblage (internal/api), la seule à connaître
// config.FidelityReal.
func (g *QueryGateway) AlreadyConfirmed(ctx context.Context, operatorID string, excludeSource bool) ([]string, error) {
	filtre := `c.operateur_id = $1`
	if excludeSource {
		filtre += ` AND c.operateur_id <> dm.operateur_source_id`
	}
	rows, err := g.db.Query(ctx, `
		SELECT dm.id FROM demande dm
		  JOIN confirmation c ON c.demande_id = dm.id
		 WHERE `+filtre+`
		 ORDER BY dm.date_demande`, operatorID)
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

func (g *QueryGateway) ToConfirm(ctx context.Context, operatorID string) ([]string, error) {
	return g.idsAConfirmer(ctx, operatorID)
}

// idsAConfirmer liste les demandes EN_COURS/CONFIRMATION où l'opérateur
// donné est un confirmateur attendu (entity.ExpectedConfirmers) et n'a pas
// encore confirmé — moved verbatim from Deps.idsAConfirmer.
func (g *QueryGateway) idsAConfirmer(ctx context.Context, operatorID string) ([]string, error) {
	// Chargé avant d'ouvrir le curseur des candidats, pour ne jamais tenir
	// deux requêtes en cours sur le pool en même temps.
	tousOps, errOps := g.tousOperateurs(ctx)
	if errOps != nil {
		return nil, errOps
	}

	rows, err := g.db.Query(ctx, `
		SELECT dm.id, dm.type_demande, dm.operateur_destinataire_id,
		       EXISTS (SELECT 1 FROM confirmation c
		                WHERE c.demande_id = dm.id AND c.operateur_id = $1)
		  FROM demande dm
		 WHERE dm.statut_demande = 'EN_COURS' AND dm.etape_actuelle = 'CONFIRMATION'
		 ORDER BY dm.date_demande`, operatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var id, typeDem, destinataireID string
		var dejaConfirme bool
		if err := rows.Scan(&id, &typeDem, &destinataireID, &dejaConfirme); err != nil {
			return nil, err
		}
		if dejaConfirme {
			continue
		}
		dm := entity.PortingRequest{
			RequestType:         entity.RequestType(typeDem),
			RecipientOperatorID: destinataireID,
		}
		attendu := false
		for _, op := range entity.ExpectedConfirmers(dm, tousOps) {
			if op == operatorID {
				attendu = true
				break
			}
		}
		if attendu {
			out = append(out, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// tousOperateurs — moved verbatim from Deps.tousOperateurs.
func (g *QueryGateway) tousOperateurs(ctx context.Context) ([]string, error) {
	rows, err := g.db.Query(ctx, `SELECT id FROM operateur ORDER BY id`)
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

// ByID — moved verbatim from Deps.detailFiltre for the two queues whose
// membership is a plain SQL predicate, and reuses idsAConfirmer's Go-side
// rule for a-confirmer, exactly as Deps.detailParmi scanned idsAConfirmer's
// result before this task.
func (g *QueryGateway) ByID(ctx context.Context, queue port.Queue, id, operatorID string) (entity.PortingRequest, bool, error) {
	switch queue {
	case port.QueueToAccept:
		return g.byFiltre(ctx, filtreAAccepter(), id, operatorID)
	case port.QueueToProcess:
		return g.byFiltre(ctx, filtreATraiter(), id, operatorID)
	case port.QueueToConfirm:
		return g.byConfirmer(ctx, id, operatorID)
	default:
		return entity.PortingRequest{}, false, fmt.Errorf("file de lecture inconnue : %q", queue)
	}
}

// byFiltre réutilise le filtre d'une liste réduite à une clause WHERE sur
// demande : une demande qui ne le satisfait pas n'existe pas pour cet
// endpoint — moved verbatim from Deps.detailFiltre's query.
func (g *QueryGateway) byFiltre(ctx context.Context, filtre, id, operatorID string) (entity.PortingRequest, bool, error) {
	var trouve bool
	err := g.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM demande dm WHERE dm.id = $2 AND `+filtre+`)`,
		operatorID, id).Scan(&trouve)
	if err != nil {
		return entity.PortingRequest{}, false, err
	}
	if !trouve {
		return entity.PortingRequest{}, false, nil
	}
	return entity.PortingRequest{ID: id}, true, nil
}

// byConfirmer réutilise idsAConfirmer, dont le filtre n'est pas une simple
// clause SQL — moved verbatim from Deps.detailParmi's scan.
func (g *QueryGateway) byConfirmer(ctx context.Context, id, operatorID string) (entity.PortingRequest, bool, error) {
	ids, err := g.idsAConfirmer(ctx, operatorID)
	if err != nil {
		return entity.PortingRequest{}, false, err
	}
	for _, x := range ids {
		if x == id {
			return entity.PortingRequest{ID: id}, true, nil
		}
	}
	return entity.PortingRequest{}, false, nil
}
