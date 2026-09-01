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

// Cancel withdraws a request before it has moved — moved verbatim from the
// deleted internal/api/annulation.go's postAnnuler, which opened its own
// *pgx.Tx for these same two statements. Unlike Reject, there is no
// rejection reason and no commentaire to record, but transition_prevue_a is
// cleared: a request cancelled mid-convergence must not have the engine
// apply a transition onto a demande that no longer exists as EN_COURS.
func (g *RequestGateway) Cancel(ctx context.Context, requestID, operatorID string, now time.Time) error {
	if _, err := g.db.Exec(ctx,
		`INSERT INTO etape_historique
		   (demande_id, etape, statut, operateur_id, origine, commentaire, date_debut, date_fin)
		 SELECT id, etape_actuelle, 'TERMINE', $2, 'ACTION', NULL, date_debut_etape, $3
		   FROM demande WHERE id = $1`,
		requestID, operatorID, now); err != nil {
		return err
	}

	_, err := g.db.Exec(ctx,
		`UPDATE demande
		    SET statut_demande = 'ANNULE', statut_etape_actuel = 'TERMINE',
		        date_finalisation = $2, transition_prevue_a = NULL
		  WHERE id = $1`,
		requestID, now)
	return err
}

// LockForTransition reads a request's transition-relevant fields with a row
// lock — moved verbatim from the deleted internal/engine/transitions.go's
// AppliquerTransition, whose own SELECT ... FOR UPDATE opened every
// transition directly against a *pgx.Tx.
func (g *RequestGateway) LockForTransition(ctx context.Context, id string) (entity.PortingRequest, bool, error) {
	var dm entity.PortingRequest
	var etape, statutDem, typeDem string
	err := g.db.QueryRow(ctx,
		`SELECT etape_actuelle, statut_demande, type_demande,
		        operateur_source_id, operateur_destinataire_id, numero
		   FROM demande WHERE id = $1 FOR UPDATE`, id).
		Scan(&etape, &statutDem, &typeDem, &dm.SourceOperatorID, &dm.RecipientOperatorID, &dm.MSISDN)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.PortingRequest{}, false, nil
	}
	if err != nil {
		return entity.PortingRequest{}, false, err
	}
	dm.ID = id
	dm.CurrentStep = entity.Step(etape)
	dm.Status = entity.RequestStatus(statutDem)
	dm.RequestType = entity.RequestType(typeDem)
	return dm, true, nil
}

// CloseCurrentStep writes the etape_historique row that closes id's current
// step — AppliquerTransition's own first tx.Exec.
func (g *RequestGateway) CloseCurrentStep(ctx context.Context, id string, closedStatus entity.StepStatus, origin string, now time.Time) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO etape_historique (demande_id, etape, statut, origine, date_debut, date_fin)
		 SELECT id, etape_actuelle, $2, $3, date_debut_etape, $4 FROM demande WHERE id = $1`,
		id, string(closedStatus), origin, now)
	return err
}

// CompleteRequest marks id TERMINE — AppliquerTransition's own COMPLETION
// branch.
func (g *RequestGateway) CompleteRequest(ctx context.Context, id string, closedStatus entity.StepStatus, now time.Time) error {
	_, err := g.db.Exec(ctx,
		`UPDATE demande
		    SET statut_demande = 'TERMINE', statut_etape_actuel = $2,
		        date_finalisation = $3, transition_prevue_a = NULL
		  WHERE id = $1`, id, string(closedStatus), now)
	return err
}

// AdvanceStep moves id to its next step, EN_COURS — AppliquerTransition's
// own non-terminal branch.
func (g *RequestGateway) AdvanceStep(ctx context.Context, id string, next entity.Step, now time.Time) error {
	_, err := g.db.Exec(ctx,
		`UPDATE demande
		    SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		        date_debut_etape = $3, transition_prevue_a = NULL
		  WHERE id = $1`, id, string(next), now)
	return err
}

// TransferToRegistry inscrit le changement d'opérateur au registre national
// — transfererAuRegistre, moved verbatim. C'est le constat central du SIT :
// quand une étape expire, ce transfert a lieu alors qu'aucun HLR n'a été
// touché. Le filtre NOT exclu AND statut <> 'REJETE' est la garantie
// centrale : sans lui, un numéro exclu ou rejeté serait transféré vers un
// opérateur que l'abonné n'a jamais accepté (TestTransfertRegistreExclutNumerosExclusEtRejetes).
func (g *RequestGateway) TransferToRegistry(ctx context.Context, id, recipientOperatorID string) error {
	_, err := g.db.Exec(ctx,
		`UPDATE numero SET operateur_actuel_id = $2, date_dernier_portage = now()
		  WHERE msisdn IN (SELECT numero FROM demande_numero
		                    WHERE demande_id = $1 AND NOT exclu AND statut <> 'REJETE')`,
		id, recipientOperatorID)
	return err
}

// ApplyRouting finalise le routage numéro par numéro (§7.10) — recalculerRoutage,
// minus its two RoutingPrefix reads, already made by the caller.
func (g *RequestGateway) ApplyRouting(ctx context.Context, id, sourcePrefix, recipientPrefix string) error {
	if _, err := g.db.Exec(ctx,
		`UPDATE demande_numero
		    SET routage_info = CASE WHEN statut = 'REJETE' OR exclu THEN $2 ELSE $3 END
		  WHERE demande_id = $1`, id, sourcePrefix, recipientPrefix); err != nil {
		return err
	}
	_, err := g.db.Exec(ctx, `UPDATE demande SET routage_info = $2 WHERE id = $1`, id, recipientPrefix)
	return err
}

// ApplyEndOfRequestRestitution : pour une RESTITUTION ou un REVERSE, le
// numéro rejoint son opérateur d'origine et routageInfo n'apparaît qu'ici
// (§7.10) — effetsFinDeDemande, minus its own RoutingPrefix read, already
// made by the caller.
func (g *RequestGateway) ApplyEndOfRequestRestitution(ctx context.Context, id, msisdn, recipientOperatorID, recipientPrefix string) error {
	if _, err := g.db.Exec(ctx,
		`UPDATE numero
		    SET operateur_actuel_id = $2, date_dernier_portage = now(), deja_restitue = true
		  WHERE msisdn = $1`, msisdn, recipientOperatorID); err != nil {
		return err
	}
	_, err := g.db.Exec(ctx, `UPDATE demande SET routage_info = $2 WHERE id = $1`, id, recipientPrefix)
	return err
}

// ScheduleTransitionAt marks id's current step processed and fixes the
// instant its transition will actually apply. L'échéance est calculée par
// la base, pas par Go : c'est le now() de Postgres que DueConvergences
// relira, et deux horloges pour une même comparaison produisent une
// intermittence (le conteneur Postgres et le process Go ne s'accordent pas
// à la milliseconde près) — commit 94af3f2.
func (g *RequestGateway) ScheduleTransitionAt(ctx context.Context, id string, delaySeconds float64) error {
	_, err := g.db.Exec(ctx,
		`UPDATE demande SET transition_prevue_a = now() + make_interval(secs => $2)
		  WHERE id = $1`, id, delaySeconds)
	return err
}

// DueConvergences lists the ids whose deferred transition has come due —
// appliquerConvergencesDues's own SELECT, moved verbatim.
func (g *RequestGateway) DueConvergences(ctx context.Context) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM demande
		  WHERE statut_demande = 'EN_COURS'
		    AND transition_prevue_a IS NOT NULL
		    AND transition_prevue_a <= now()`)
	if err != nil {
		return nil, err
	}
	return scanIDs(rows)
}

// OverdueSteps lists the ids whose current step has run past timeoutSeconds
// without a pending transition — expirerEtapes's own SELECT, moved verbatim.
func (g *RequestGateway) OverdueSteps(ctx context.Context, timeoutSeconds float64, asOf time.Time) ([]string, error) {
	rows, err := g.db.Query(ctx,
		`SELECT id FROM demande
		  WHERE statut_demande = 'EN_COURS'
		    AND transition_prevue_a IS NULL
		    AND date_debut_etape + make_interval(secs => $1) <= $2`,
		timeoutSeconds, asOf)
	if err != nil {
		return nil, err
	}
	return scanIDs(rows)
}

// CreateAtConfirmation inserts a new request directly at CONFIRMATION —
// ValiderReverse's own INSERT (§6), moved verbatim: a REVERSE never goes
// through ACCEPTATION or DESACTIVATION/ACTIVATION.
func (g *RequestGateway) CreateAtConfirmation(ctx context.Context, in port.CreateRequestInput) error {
	_, err := g.db.Exec(ctx,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, date_demande, date_debut_etape)
		 VALUES ($1,$2,$3,$4,'EN_COURS','CONFIRMATION','EN_COURS',$5,$6,$7,$8,$8)`,
		in.ID, in.MSISDN, in.SubscriberType, in.RequestType,
		in.SourceOperatorID, in.RecipientOperatorID, in.CreatorOperatorID, in.RequestDate)
	return err
}

// PendingReverseCompletions lists the REVERSE requests
// completerReversesConfirmes must catch up — moved verbatim; see that
// function's own deleted doc comment (internal/engine/reverse.go) for why
// both branches of the OR are necessary.
func (g *RequestGateway) PendingReverseCompletions(ctx context.Context) ([]port.PendingReverseCompletion, error) {
	rows, err := g.db.Query(ctx,
		`SELECT d.id, d.etape_actuelle FROM demande d
		  WHERE d.type_demande = 'REVERSE'
		    AND d.statut_demande = 'EN_COURS'
		    AND d.transition_prevue_a IS NULL
		    AND (
		         d.etape_actuelle = 'COMPLETION'
		      OR (d.etape_actuelle = 'CONFIRMATION'
		          AND (SELECT count(*) FROM confirmation c WHERE c.demande_id = d.id)
		              >= (SELECT count(*) FROM operateur))
		    )`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []port.PendingReverseCompletion{}
	for rows.Next() {
		var c port.PendingReverseCompletion
		var etape string
		if err := rows.Scan(&c.RequestID, &etape); err != nil {
			return nil, err
		}
		c.CurrentStep = entity.Step(etape)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scanIDs drains rows of a single text column into a slice — the shape
// DueConvergences and OverdueSteps both need, moved verbatim from the
// deleted internal/engine/engine.go's own repeated inline loop.
func scanIDs(rows pgx.Rows) ([]string, error) {
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
