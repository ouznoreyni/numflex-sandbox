package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// IncidentGateway is the Postgres implementation of port.IncidentGateway.
// Its methods carry the SQL that used to live in the deleted
// internal/api/incidents.go, unchanged. This is the one place allowed to
// know that Postgres reports the migration's partial unique index
// (incident_interne_unique_ouvert) as code 23505 — translated here into
// port.ErrIncidentAlreadyOpen, exactly as ConfirmationGateway.Confirm
// already does for its own anti-replay guarantee.
type IncidentGateway struct {
	db Querier
}

// NewIncidentGateway returns a gateway bound to db — a pool for TypeIDFor,
// HasOpen, ByID, Get and Own, or a transaction handed out by the unit of
// work for Create and Resolve.
func NewIncidentGateway(db Querier) *IncidentGateway {
	return &IncidentGateway{db: db}
}

func (g *IncidentGateway) TypeIDFor(ctx context.Context, systemLocked bool) (string, error) {
	var typeID string
	err := g.db.QueryRow(ctx,
		`SELECT id FROM type_incident WHERE fige_systeme = $1 LIMIT 1`, systemLocked).
		Scan(&typeID)
	return typeID, err
}

func (g *IncidentGateway) HasOpen(ctx context.Context, operatorID string) (bool, error) {
	var dejaOuvert bool
	err := g.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM incident
		   WHERE operateur_id = $1 AND statut = 'EN_COURS' AND fige_systeme)`,
		operatorID).Scan(&dejaOuvert)
	return dejaOuvert, err
}

func (g *IncidentGateway) Create(ctx context.Context, in port.IncidentCreateInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO incident
		   (id, operateur_id, type_incident_id, fige_systeme, description, statut, date_ouverture)
		 VALUES ($1,$2,$3,$4,$5,'EN_COURS',$6)`,
		in.ID, in.OperatorID, in.TypeID, in.SystemLocked, in.Description, in.OpenedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return port.ErrIncidentAlreadyOpen
		}
		return err
	}
	return nil
}

// ByID reads an incident's authorization-relevant shape — moved from the
// deleted internal/api/incidents.go's own inline query in resoudreIncident.
func (g *IncidentGateway) ByID(ctx context.Context, id string) (entity.Incident, bool, error) {
	var inc entity.Incident
	inc.ID = id
	err := g.db.QueryRow(ctx,
		`SELECT operateur_id, fige_systeme FROM incident WHERE id = $1`, id).
		Scan(&inc.OperatorID, &inc.SystemLocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Incident{}, false, nil
	}
	if err != nil {
		return entity.Incident{}, false, err
	}
	return inc, true, nil
}

func (g *IncidentGateway) Resolve(ctx context.Context, id, comment string, now time.Time) error {
	_, err := g.db.Exec(ctx,
		`UPDATE incident SET statut = 'RESOLU', date_resolution = $2, commentaire_resolution = $3
		  WHERE id = $1`, id, now, comment)
	return err
}

// Get reads an incident back — moved verbatim from the deleted
// internal/api/incidents.go's incidentDTO.
func (g *IncidentGateway) Get(ctx context.Context, id string) (port.IncidentView, bool, error) {
	var (
		typeID, typeLibelle, description, statut string
		figeSysteme                              bool
		dateOuverture                            time.Time
		operateurID, operateurNom                string
	)
	err := g.db.QueryRow(ctx, `
		SELECT inc.type_incident_id, ti.libelle, inc.fige_systeme, inc.description,
		       inc.statut, inc.date_ouverture, op.id, op.nom
		  FROM incident inc
		  JOIN type_incident ti ON ti.id = inc.type_incident_id
		  JOIN operateur op ON op.id = inc.operateur_id
		 WHERE inc.id = $1`, id).Scan(
		&typeID, &typeLibelle, &figeSysteme, &description,
		&statut, &dateOuverture, &operateurID, &operateurNom)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.IncidentView{}, false, nil
	}
	if err != nil {
		return port.IncidentView{}, false, err
	}
	return port.IncidentView{
		ID: id, TypeID: typeID, TypeLabel: typeLibelle, SystemLocked: figeSysteme,
		Description: description, Status: statut, OpenedAt: dateOuverture,
		OperatorID: operateurID, OperatorName: operateurNom,
	}, true, nil
}

// Own lists, paginated and in chronological order, operatorID's own
// incidents for one segment — moved verbatim from the deleted
// internal/api/incidents.go's idsIncidents.
func (g *IncidentGateway) Own(ctx context.Context, operatorID string, systemLocked bool, page, size int) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM incident
		  WHERE operateur_id = $1 AND fige_systeme = $2
		  ORDER BY date_ouverture
		  LIMIT $3 OFFSET $4`, operatorID, systemLocked, size, page*size)
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
