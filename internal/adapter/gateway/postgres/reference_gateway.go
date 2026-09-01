package postgres

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// ReferenceGateway is the Postgres implementation of port.ReferenceGateway.
// Its five methods carry the SQL that used to live in
// internal/api/referentiels.go, unchanged.
type ReferenceGateway struct {
	db Querier
}

// NewReferenceGateway returns a gateway bound to db — a pool for ad hoc use,
// or a transaction handed out by the unit of work.
func NewReferenceGateway(db Querier) *ReferenceGateway {
	return &ReferenceGateway{db: db}
}

func (g *ReferenceGateway) Operators(ctx context.Context) ([]entity.Operator, error) {
	rows, err := g.db.Query(ctx, `SELECT id, nom FROM operateur ORDER BY nom`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.Operator{}
	for rows.Next() {
		var o entity.Operator
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	// Next() returning false means either "end of rows" or "error": without
	// this check, a failure mid-iteration would pass for a partial success.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (g *ReferenceGateway) RejectionReasons(ctx context.Context) ([]entity.RejectionReason, error) {
	rows, err := g.db.Query(ctx, `SELECT id, motif FROM motif_rejet ORDER BY motif`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.RejectionReason{}
	for rows.Next() {
		var m entity.RejectionReason
		if err := rows.Scan(&m.ID, &m.Reason); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// RejectionReasonExists answers whether id names a row in motif_rejet —
// moved verbatim from internal/api/acceptation.go's motifExiste (Task 14).
func (g *ReferenceGateway) RejectionReasonExists(ctx context.Context, id string) (bool, error) {
	var existe bool
	err := g.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM motif_rejet WHERE id = $1)`, id).Scan(&existe)
	return existe, err
}

func (g *ReferenceGateway) RequestTypes(ctx context.Context) ([]entity.RequestTypeRef, error) {
	rows, err := g.db.Query(ctx, `SELECT id, type FROM type_demande ORDER BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.RequestTypeRef{}
	for rows.Next() {
		var t entity.RequestTypeRef
		if err := rows.Scan(&t.ID, &t.Type); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (g *ReferenceGateway) Processes(ctx context.Context) ([]entity.Process, error) {
	rows, err := g.db.Query(ctx, `SELECT id, type FROM processus ORDER BY type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.Process{}
	for rows.Next() {
		var p entity.Process
		if err := rows.Scan(&p.ID, &p.Type); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (g *ReferenceGateway) IncidentTypes(ctx context.Context) ([]entity.IncidentType, error) {
	rows, err := g.db.Query(ctx, `SELECT id, libelle, fige_systeme FROM type_incident ORDER BY libelle`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.IncidentType{}
	for rows.Next() {
		var ti entity.IncidentType
		if err := rows.Scan(&ti.ID, &ti.Label, &ti.SystemLocked); err != nil {
			return nil, err
		}
		out = append(out, ti)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
