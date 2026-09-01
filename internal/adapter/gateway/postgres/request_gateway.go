package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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
