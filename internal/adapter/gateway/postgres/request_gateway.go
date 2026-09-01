package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// RequestGateway is the Postgres implementation of port.RequestGateway. Its
// methods carry the SQL that used to live in
// internal/api/demandes_creation.go, unchanged in substance: the three
// original variants of the demande/demande_numero/demande_client INSERTs
// (particulier, entreprise, restitution) differed only in which columns
// they left NULL by omission, which this gateway now expresses as a nil
// pointer argument to one parameterized statement per table instead of
// three near-duplicate statements — the rows written to Postgres are
// identical either way.
type RequestGateway struct {
	db Querier
}

// NewRequestGateway returns a gateway bound to db — a pool for the post-
// commit Get, or a transaction handed out by the unit of work for
// everything that writes.
func NewRequestGateway(db Querier) *RequestGateway {
	return &RequestGateway{db: db}
}

// RoutingPrefix reads prefixe_routage, the SELECT that used to open every
// creation transaction. Any error — not found included — is for the caller
// to turn into entity.ValidationFailed("Opérateur source inconnu"), exactly
// as the legacy handlers did with tx.QueryRow(...).Scan.
func (g *RequestGateway) RoutingPrefix(ctx context.Context, operatorID string) (string, error) {
	var prefix string
	err := g.db.QueryRow(ctx,
		`SELECT prefixe_routage FROM operateur WHERE id = $1`, operatorID).Scan(&prefix)
	return prefix, err
}

func (g *RequestGateway) Create(ctx context.Context, in port.CreateRequestInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,$2,$3,$4,'EN_COURS','ACCEPTATION','EN_COURS',$5,$6,$7,$8,$9,$10,$10)`,
		in.ID, in.MSISDN, in.SubscriberType, in.RequestType,
		in.SourceOperatorID, in.RecipientOperatorID, in.CreatorOperatorID,
		in.Processus, in.RoutingInfo, in.RequestDate)
	return err
}

func (g *RequestGateway) AddNumber(ctx context.Context, in port.RequestNumberInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO demande_numero (demande_id, numero, statut, routage_info)
		 VALUES ($1,$2,'EN_COURS',$3)`,
		in.RequestID, in.MSISDN, in.RoutingInfo)
	return err
}

func (g *RequestGateway) AddExcludedNumber(ctx context.Context, in port.ExcludedNumberInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO demande_numero
		   (demande_id, numero, statut, exclu, raison_exclusion, code_erreur_exclusion)
		 VALUES ($1,$2,'REJETE',true,$3,$4)`,
		in.RequestID, in.MSISDN, in.Reason, in.ErrorCode)
	return err
}

func (g *RequestGateway) AddClient(ctx context.Context, in port.ClientInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO demande_client
		   (demande_id, nom, prenom, date_naissance, lieu_naissance, type_piece, numero_piece,
		    raison_sociale, num_rc)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		in.RequestID, in.LastName, in.FirstName, in.BirthDate, in.BirthPlace,
		in.IDType, in.IDNumber, in.CompanyName, in.RCNumber)
	return err
}

// Get reads a request back exactly as the legacy handlers' demandeDTO did —
// same SELECT, same JOINs, same three-field client-presence rule — but
// returns a typed port.RequestView instead of a map: the map-with-Skew
// formatting that used to happen inline is now the controller's job, not
// this gateway's.
func (g *RequestGateway) Get(ctx context.Context, id string) (port.RequestView, bool, error) {
	var (
		numero, typeAbonne, typeDemande, statutDemande string
		etapeActuelle, statutEtapeActuel               string
		srcID, srcNom, dstID, dstNom                   string
		dateDemande                                    time.Time
		processus, routageInfo                         sql.NullString
		dateFinalisation                               sql.NullTime
		cliNom, cliPrenom, cliLieu, cliPiece, cliNum   sql.NullString
		cliNaissance                                   sql.NullTime
	)

	err := g.db.QueryRow(ctx, `
		SELECT dem.numero, dem.type_abonne, dem.type_demande, dem.statut_demande,
		       dem.etape_actuelle, dem.statut_etape_actuel,
		       src.id, src.nom, dst.id, dst.nom,
		       dem.date_demande, dem.processus, dem.routage_info, dem.date_finalisation,
		       cli.nom, cli.prenom, cli.date_naissance, cli.lieu_naissance,
		       cli.type_piece, cli.numero_piece
		  FROM demande dem
		  JOIN operateur src ON src.id = dem.operateur_source_id
		  JOIN operateur dst ON dst.id = dem.operateur_destinataire_id
		  LEFT JOIN demande_client cli ON cli.demande_id = dem.id
		 WHERE dem.id = $1`, id).Scan(
		&numero, &typeAbonne, &typeDemande, &statutDemande,
		&etapeActuelle, &statutEtapeActuel,
		&srcID, &srcNom, &dstID, &dstNom,
		&dateDemande, &processus, &routageInfo, &dateFinalisation,
		&cliNom, &cliPrenom, &cliNaissance, &cliLieu, &cliPiece, &cliNum)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.RequestView{}, false, nil
	}
	if err != nil {
		return port.RequestView{}, false, err
	}

	view := port.RequestView{
		ID: id, MSISDN: numero,
		SubscriberType: typeAbonne, RequestType: typeDemande, Status: statutDemande,
		CurrentStep: etapeActuelle, CurrentStepStatus: statutEtapeActuel,
		SourceOperatorID: srcID, SourceOperatorName: srcNom,
		RecipientOperatorID: dstID, RecipientOperatorName: dstNom,
		RequestDate: dateDemande,
	}
	if processus.Valid {
		v := processus.String
		view.Processus = &v
	}
	if routageInfo.Valid {
		v := routageInfo.String
		view.RoutingInfo = &v
	}
	if dateFinalisation.Valid {
		v := dateFinalisation.Time
		view.CompletionDate = &v
	}
	// Le client est rendu dans toutes les captures — création, acceptation,
	// traitement, a-traiter, in — avec exactement ces six champs. Sa présence
	// se décide sur ces trois colonnes, comme demandeDTO le faisait.
	if cliNom.Valid || cliPrenom.Valid || cliNum.Valid {
		client := &port.ClientView{
			LastName: cliNom.String, FirstName: cliPrenom.String,
			BirthPlace: cliLieu.String, IDType: cliPiece.String, IDNumber: cliNum.String,
		}
		if cliNaissance.Valid {
			t := cliNaissance.Time
			client.BirthDate = &t
		}
		view.Client = client
	}
	return view, true, nil
}

// ByID reads a request's authorization-relevant shape — moved verbatim from
// internal/api/dto.go's chargerDemande (Task 14: acceptance).
func (g *RequestGateway) ByID(ctx context.Context, id string) (entity.PortingRequest, bool, error) {
	var dm entity.PortingRequest
	var typeDemande, typeAbonne, statutDemande, etape, statutEtape string
	var transition *string

	err := g.db.QueryRow(ctx,
		`SELECT id, numero, type_demande, type_abonne, statut_demande, etape_actuelle,
		        statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		        createur_operateur_id, transition_prevue_a::text
		   FROM demande WHERE id = $1`, id).
		Scan(&dm.ID, &dm.MSISDN, &typeDemande, &typeAbonne, &statutDemande, &etape, &statutEtape,
			&dm.SourceOperatorID, &dm.RecipientOperatorID, &dm.CreatorOperatorID, &transition)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.PortingRequest{}, false, nil
	}
	if err != nil {
		return entity.PortingRequest{}, false, err
	}

	dm.RequestType = entity.RequestType(typeDemande)
	dm.SubscriberType = entity.SubscriberType(typeAbonne)
	dm.Status = entity.RequestStatus(statutDemande)
	dm.CurrentStep = entity.Step(etape)
	dm.CurrentStepStatus = entity.StepStatus(statutEtape)
	dm.PendingTransition = transition != nil
	return dm, true, nil
}

// SetComment writes commentaire alone, leaving every other column of the
// request untouched.
func (g *RequestGateway) SetComment(ctx context.Context, id, comment string) error {
	_, err := g.db.Exec(ctx,
		`UPDATE demande SET commentaire = NULLIF($2, '') WHERE id = $1`, id, comment)
	return err
}

// NumberBelongs answers whether msisdn is one of requestID's demande_numero
// rows.
func (g *RequestGateway) NumberBelongs(ctx context.Context, requestID, msisdn string) (bool, error) {
	var appartient bool
	err := g.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM demande_numero WHERE demande_id = $1 AND numero = $2)`,
		requestID, msisdn).Scan(&appartient)
	return appartient, err
}

// RejectNumber marks one fleet member REJETE.
func (g *RequestGateway) RejectNumber(ctx context.Context, requestID, msisdn, rejectionReasonID string) error {
	_, err := g.db.Exec(ctx,
		`UPDATE demande_numero SET statut = 'REJETE', motif_rejet_id = NULLIF($3, '')
		  WHERE demande_id = $1 AND numero = $2`,
		requestID, msisdn, rejectionReasonID)
	return err
}

// HasActiveNumber answers whether requestID still has at least one
// demande_numero row that is not REJETE.
func (g *RequestGateway) HasActiveNumber(ctx context.Context, requestID string) (bool, error) {
	var resteEligible bool
	err := g.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM demande_numero WHERE demande_id = $1 AND statut <> 'REJETE')`,
		requestID).Scan(&resteEligible)
	return resteEligible, err
}

// Reject closes a request definitively — moved verbatim from
// internal/api/acceptation.go's rejeterDemande: the etape_historique row
// that no engine transition will ever write for a rejection, then the
// demande row itself.
func (g *RequestGateway) Reject(ctx context.Context, requestID, operatorID, rejectionReasonID,
	comment string, now time.Time) error {

	if _, err := g.db.Exec(ctx,
		`INSERT INTO etape_historique
		   (demande_id, etape, statut, operateur_id, origine, commentaire, date_debut, date_fin)
		 SELECT id, etape_actuelle, 'TERMINE', $2, 'ACTION', NULLIF($3, ''), date_debut_etape, $4
		   FROM demande WHERE id = $1`,
		requestID, operatorID, comment, now); err != nil {
		return err
	}

	_, err := g.db.Exec(ctx,
		`UPDATE demande
		    SET statut_demande = 'REJETE', statut_etape_actuel = 'TERMINE',
		        date_finalisation = $2, motif_rejet_id = NULLIF($3, ''), commentaire = NULLIF($4, '')
		  WHERE id = $1`,
		requestID, now, rejectionReasonID, comment)
	return err
}
