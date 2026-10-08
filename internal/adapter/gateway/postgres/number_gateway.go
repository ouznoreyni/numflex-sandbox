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

// NewNumberGateway returns a gateway bound to db. The registry is the numero
// table and nothing else: a number exists only if a row says so.
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

// Register writes msisdn into the registry at operatorID, at home and never
// ported. ON CONFLICT leaves a number already there exactly as it is, and an
// operatorID the operateur table does not know writes nothing.
func (g *NumberGateway) Register(ctx context.Context, msisdn, operatorID string) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO numero (msisdn, operateur_actuel_id, operateur_origine_id)
		 SELECT $1, id, id FROM operateur WHERE id = $2
		 ON CONFLICT (msisdn) DO NOTHING`, msisdn, operatorID)
	return err
}
