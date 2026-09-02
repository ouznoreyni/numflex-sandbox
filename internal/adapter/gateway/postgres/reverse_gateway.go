package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ReverseGateway is the Postgres implementation of port.ReverseGateway. Its
// methods carry the SQL that used to live in the deleted
// internal/api/reverse.go, unchanged.
type ReverseGateway struct {
	db Querier
}

// NewReverseGateway returns a gateway bound to db — a pool for Get and Own,
// or a transaction handed out by the unit of work for Create.
func NewReverseGateway(db Querier) *ReverseGateway {
	return &ReverseGateway{db: db}
}

func (g *ReverseGateway) Create(ctx context.Context, in port.ReverseCreateInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO reverse_request (id, numero, operateur_id, statut, date_demande)
		 VALUES ($1,$2,$3,'EN_ATTENTE',$4)`,
		in.ID, in.MSISDN, in.OperatorID, in.RequestDate)
	return err
}

// Get reads a reverse request back — moved verbatim from the deleted
// internal/api/reverse.go's reverseRequestDTO.
func (g *ReverseGateway) Get(ctx context.Context, id string) (port.ReverseView, bool, error) {
	var msisdn, status, operatorID, operatorName string
	var requestDate time.Time
	err := g.db.QueryRow(ctx, `
		SELECT rr.numero, rr.statut, rr.date_demande, op.id, op.nom
		  FROM reverse_request rr
		  JOIN operateur op ON op.id = rr.operateur_id
		 WHERE rr.id = $1`, id).
		Scan(&msisdn, &status, &requestDate, &operatorID, &operatorName)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.ReverseView{}, false, nil
	}
	if err != nil {
		return port.ReverseView{}, false, err
	}
	return port.ReverseView{
		ID: id, MSISDN: msisdn, Status: status, RequestDate: requestDate,
		OperatorID: operatorID, OperatorName: operatorName,
	}, true, nil
}

// Own lists, paginated and in chronological order, operatorID's own reverse
// requests — moved verbatim from the deleted internal/api/reverse.go's
// idsReverseRequests.
func (g *ReverseGateway) Own(ctx context.Context, operatorID string, page, size int) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM reverse_request
		  WHERE operateur_id = $1
		  ORDER BY date_demande
		  LIMIT $2 OFFSET $3`, operatorID, size, page*size)
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

// LockPending reads a reverse request's number, operator and status with a
// row lock — moved verbatim from the deleted internal/engine/reverse.go's
// ValidateReverse, whose own SELECT ... FOR UPDATE opened every validation
// directly against a *pgx.Tx.
func (g *ReverseGateway) LockPending(ctx context.Context, id string) (msisdn, operatorID, status string, err error) {
	err = g.db.QueryRow(ctx,
		`SELECT numero, operateur_id, statut FROM reverse_request WHERE id = $1 FOR UPDATE`,
		id).Scan(&msisdn, &operatorID, &status)
	return msisdn, operatorID, status, err
}

// MarkValidated records that id was validated into requestID — moved
// verbatim from ValidateReverse's own final tx.Exec.
func (g *ReverseGateway) MarkValidated(ctx context.Context, id, requestID string, now time.Time) error {
	_, err := g.db.Exec(ctx,
		`UPDATE reverse_request SET statut='VALIDE', date_decision=$2, demande_id=$3
		  WHERE id = $1`, id, now, requestID)
	return err
}

// Reject marks id REJETE without creating any Demande — moved verbatim from
// the deleted internal/engine/reverse.go's RejectReverse.
func (g *ReverseGateway) Reject(ctx context.Context, id string) error {
	_, err := g.db.Exec(ctx,
		`UPDATE reverse_request SET statut='REJETE', date_decision=now()
		  WHERE id = $1 AND statut = 'EN_ATTENTE'`, id)
	return err
}

// CurrentOperatorFor reads a number's current holder — the one field
// ValidateReverse needs from the registry, kept here (rather than on
// NumberGateway) so it stays inside the same transaction as LockPending's
// own lock.
func (g *ReverseGateway) CurrentOperatorFor(ctx context.Context, msisdn string) (string, error) {
	var currentOperator string
	err := g.db.QueryRow(ctx,
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = $1`, msisdn).Scan(&currentOperator)
	return currentOperator, err
}

// OverdueForAutoValidation lists the ids EN_ATTENTE for more than
// delaySeconds — validerReversesAutomatiquement's own SELECT, moved
// verbatim.
func (g *ReverseGateway) OverdueForAutoValidation(ctx context.Context, delaySeconds float64) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM reverse_request
		  WHERE statut = 'EN_ATTENTE' AND date_demande + make_interval(secs => $1) <= now()`,
		delaySeconds)
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
