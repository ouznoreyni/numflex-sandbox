package postgres

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// NumberGateway is the Postgres implementation of port.NumberGateway. Its
// one method carries the SQL that used to live in internal/api/dto.go's
// etatNumero, unchanged — this task's own copy, since etatNumero stays in
// internal/api for the handlers that have not migrated yet (acceptation,
// annulation, confirmation, lecture, traitement, reverse).
type NumberGateway struct {
	db    Querier
	homes map[string]string
}

// NewNumberGateway returns a gateway bound to db — always the plain pool in
// this task, since every number-state read happens before request
// creation's transaction opens.
//
// homes maps a three-digit prefix to the operator whose home range it is.
// Every well-formed number of such a range exists, seeded or not: the
// registry is open on them, and a number the seed never wrote is written on
// its first read, at home and never ported. Any other number exists only if
// a row says so.
func NewNumberGateway(db Querier, homes map[string]string) *NumberGateway {
	return &NumberGateway{db: db, homes: homes}
}

// wellFormed is the national format: nine digits, a three-digit prefix and
// a six-digit tail.
var wellFormed = regexp.MustCompile(`^[0-9]{9}$`)

func (g *NumberGateway) State(ctx context.Context, msisdn string) (entity.NumberState, bool, error) {
	if err := g.openHomeNumber(ctx, msisdn); err != nil {
		return entity.NumberState{}, false, err
	}

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

// openHomeNumber writes msisdn into the registry when it belongs to a home
// range and is not there yet. ON CONFLICT leaves a number that already has a
// history — ported, restituted, seeded — exactly as it is.
func (g *NumberGateway) openHomeNumber(ctx context.Context, msisdn string) error {
	if !wellFormed.MatchString(msisdn) {
		return nil
	}
	operator, ok := g.homes[msisdn[:3]]
	if !ok {
		return nil
	}
	_, err := g.db.Exec(ctx,
		`INSERT INTO numero (msisdn, operateur_actuel_id, operateur_origine_id)
		 VALUES ($1, $2, $2)
		 ON CONFLICT (msisdn) DO NOTHING`, msisdn, operator)
	return err
}
