package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// NumberGateway is the Postgres implementation of port.NumberGateway. Its
// one method carries the SQL that used to live in internal/api/dto.go's
// etatNumero, unchanged — this task's own copy, since etatNumero stays in
// internal/api for the handlers that have not migrated yet (acceptation,
// annulation, confirmation, lecture, traitement, reverse).
type NumberGateway struct {
	db Querier
}

// NewNumberGateway returns a gateway bound to db — always the plain pool in
// this task, since every number-state read happens before request
// creation's transaction opens.
func NewNumberGateway(db Querier) *NumberGateway {
	return &NumberGateway{db: db}
}

func (g *NumberGateway) State(ctx context.Context, msisdn string) (entity.NumberState, bool, error) {
	n := entity.NumberState{MSISDN: msisdn}

	err := g.db.QueryRow(ctx,
		`SELECT operateur_actuel_id, operateur_origine_id, date_dernier_portage, deja_restitue
		   FROM numero WHERE msisdn = $1`, msisdn).
		Scan(&n.CurrentOperatorID, &n.OriginOperatorID, &n.LastPortingDate, &n.AlreadyRestituted)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.NumberState{}, false, nil
	}
	if err != nil {
		return entity.NumberState{}, false, err
	}

	if err := g.db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM demande_numero dn
		    JOIN demande dm ON dm.id = dn.demande_id
		   WHERE dn.numero = $1
		     AND dm.statut_demande = 'EN_COURS'
		     AND NOT dn.exclu
		     AND dn.statut <> 'REJETE')`, msisdn).
		Scan(&n.RequestInProgress); err != nil {
		return entity.NumberState{}, false, err
	}

	return n, true, nil
}
