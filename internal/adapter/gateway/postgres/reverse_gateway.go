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
	var numero, statut, operateurID, operateurNom string
	var dateDemande time.Time
	err := g.db.QueryRow(ctx, `
		SELECT rr.numero, rr.statut, rr.date_demande, op.id, op.nom
		  FROM reverse_request rr
		  JOIN operateur op ON op.id = rr.operateur_id
		 WHERE rr.id = $1`, id).
		Scan(&numero, &statut, &dateDemande, &operateurID, &operateurNom)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.ReverseView{}, false, nil
	}
	if err != nil {
		return port.ReverseView{}, false, err
	}
	return port.ReverseView{
		ID: id, MSISDN: numero, Status: statut, RequestDate: dateDemande,
		OperatorID: operateurID, OperatorName: operateurNom,
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
