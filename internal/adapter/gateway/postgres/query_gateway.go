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

// ids runs a query whose only column is the request id, for the lists whose
// filter reduces to a WHERE clause on the demande table. Ordering by
// date_demande reproduces the chronological order expected of a work
// queue — moved verbatim from Deps.idsDemandes.
func (g *QueryGateway) ids(ctx context.Context, filter string, args ...any) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM demande dm WHERE `+filter+` ORDER BY date_demande`, args...)
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

// toAcceptFilter — moved verbatim from Deps.filtreAAccepter.
func toAcceptFilter() string {
	return `dm.statut_demande = 'EN_COURS' AND dm.etape_actuelle = 'ACCEPTATION'
	         AND dm.operateur_source_id = $1`
}

func (g *QueryGateway) ToAccept(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx, toAcceptFilter(), operatorID)
}

// toProcessFilter — moved verbatim from Deps.filtreATraiter, comment
// included: ACCEPTATION is in it: the capture « 1.orange1_EN_COURS_Demandes
// à traiter_next_ACCEPTATION » shows a request at that step in the source's
// queue. a-traiter answers "requiring action from you" (§7.7), not a
// subset of steps — a-accepter is only one specialized view of it.
func toProcessFilter() string {
	return `dm.statut_demande = 'EN_COURS' AND (
	           (dm.etape_actuelle IN ('ACCEPTATION', 'DESACTIVATION')
	            AND dm.operateur_source_id = $1)
	        OR (dm.etape_actuelle IN ('ACTIVATION', 'COMPLETION')
	            AND dm.operateur_destinataire_id = $1
	            AND NOT (dm.etape_actuelle = 'COMPLETION' AND dm.type_demande = 'REVERSE'))
	        )`
}

func (g *QueryGateway) ToProcess(ctx context.Context, operatorID string) ([]string, error) {
	return g.ids(ctx, toProcessFilter(), operatorID)
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
// ANO-019 — this is the sole place in the project where a SQL query depends
// on the fidelity mode. In real mode, the measured platform omits from this
// list the confirmations issued by the request's source operator: ORANGE,
// source of a porting, confirms successfully but its own confirmation never
// appears in /deja-confirmees; a third party's does. In contract mode this
// filter does not exist — excludeSource carries that choice down from the
// wiring layer (internal/api), the only one allowed to know
// config.FidelityReal.
func (g *QueryGateway) AlreadyConfirmed(ctx context.Context, operatorID string, excludeSource bool) ([]string, error) {
	filter := `c.operateur_id = $1`
	if excludeSource {
		filter += ` AND c.operateur_id <> dm.operateur_source_id`
	}
	rows, err := g.db.Query(ctx, `
		SELECT dm.id FROM demande dm
		  JOIN confirmation c ON c.demande_id = dm.id
		 WHERE `+filter+`
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
	return g.idsToConfirm(ctx, operatorID)
}

// idsToConfirm lists the EN_COURS/CONFIRMATION requests where the given
// operator is an expected confirmer (entity.ExpectedConfirmers) and has not
// yet confirmed — moved verbatim from Deps.idsAConfirmer.
func (g *QueryGateway) idsToConfirm(ctx context.Context, operatorID string) ([]string, error) {
	// Loaded before opening the candidates' cursor, so as to never hold two
	// queries open on the pool at once.
	allOps, errOps := g.allOperators(ctx)
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
		var id, requestType, recipientID string
		var alreadyConfirmed bool
		if err := rows.Scan(&id, &requestType, &recipientID, &alreadyConfirmed); err != nil {
			return nil, err
		}
		if alreadyConfirmed {
			continue
		}
		dm := entity.PortingRequest{
			RequestType:         entity.RequestType(requestType),
			RecipientOperatorID: recipientID,
		}
		expected := false
		for _, op := range entity.ExpectedConfirmers(dm, allOps) {
			if op == operatorID {
				expected = true
				break
			}
		}
		if expected {
			out = append(out, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// allOperators — moved verbatim from Deps.tousOperateurs.
func (g *QueryGateway) allOperators(ctx context.Context) ([]string, error) {
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
// membership is a plain SQL predicate, and reuses idsToConfirm's Go-side
// rule for a-confirmer, exactly as Deps.detailParmi scanned idsAConfirmer's
// result before this task.
func (g *QueryGateway) ByID(ctx context.Context, queue port.Queue, id, operatorID string) (entity.PortingRequest, bool, error) {
	switch queue {
	case port.QueueToAccept:
		return g.byFilter(ctx, toAcceptFilter(), id, operatorID)
	case port.QueueToProcess:
		return g.byFilter(ctx, toProcessFilter(), id, operatorID)
	case port.QueueToConfirm:
		return g.byConfirmer(ctx, id, operatorID)
	default:
		return entity.PortingRequest{}, false, fmt.Errorf("unknown read queue: %q", queue)
	}
}

// byFilter reuses a list's filter reduced to a WHERE clause on demande: a
// request that does not satisfy it does not exist for this endpoint —
// moved verbatim from Deps.detailFiltre's query.
func (g *QueryGateway) byFilter(ctx context.Context, filter, id, operatorID string) (entity.PortingRequest, bool, error) {
	var found bool
	err := g.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM demande dm WHERE dm.id = $2 AND `+filter+`)`,
		operatorID, id).Scan(&found)
	if err != nil {
		return entity.PortingRequest{}, false, err
	}
	if !found {
		return entity.PortingRequest{}, false, nil
	}
	return entity.PortingRequest{ID: id}, true, nil
}

// byConfirmer reuses idsToConfirm, whose filter is not a plain SQL clause —
// moved verbatim from Deps.detailParmi's scan.
func (g *QueryGateway) byConfirmer(ctx context.Context, id, operatorID string) (entity.PortingRequest, bool, error) {
	ids, err := g.idsToConfirm(ctx, operatorID)
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
