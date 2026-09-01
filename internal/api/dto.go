package api

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// motifMSISDN est le format MSISDN du contrat ARTP. Sa copie d'origine
// vivait dans internal/api/otp.go ; elle reste ici pour reverse.go, qui
// l'utilise encore, jusqu'à ce qu'il migre à son tour (Task 19). Le
// contrôleur OTP et le contrôleur de création (internal/adapter/controller)
// portent désormais chacun leur propre copie, indépendante.
var motifMSISDN = regexp.MustCompile(`^[0-9]{9}$`)

// demandeDTO sérialise une demande au format du guide §7.3, commun à tous les
// endpoints qui renvoient une demande (Tasks 10 à 17). Tous les horodatages
// passent par d.R.Skew() : la dérive n'existe qu'au rendu, jamais en base.
func (d *Deps) demandeDTO(ctx context.Context, id string) (map[string]any, error) {
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

	err := d.DB.Pool.QueryRow(ctx, `
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
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"id":                    id,
		"numero":                numero,
		"typeAbonne":            typeAbonne,
		"typeDemande":           typeDemande,
		"statutDemande":         statutDemande,
		"etapeActuelle":         etapeActuelle,
		"statutEtapeActuel":     statutEtapeActuel,
		"operateurSource":       map[string]any{"id": srcID, "nom": srcNom},
		"operateurDestinataire": map[string]any{"id": dstID, "nom": dstNom},
		"dateDemande":           d.R.Skew(dateDemande),
		"processus":             nil,
		"routageInfo":           nil,
	}
	if processus.Valid {
		out["processus"] = processus.String
	}
	if routageInfo.Valid {
		out["routageInfo"] = routageInfo.String
	}
	if dateFinalisation.Valid {
		out["dateFinalisation"] = d.R.Skew(dateFinalisation.Time)
	}

	// Le client est rendu dans toutes les captures — création, acceptation,
	// traitement, a-traiter, in — avec exactement ces six champs, et sans
	// raisonSociale ni numRC même sur une flotte. Il est absent des seules
	// réponses de confirmation, que sansClient dépouille.
	if cliNom.Valid || cliPrenom.Valid || cliNum.Valid {
		client := map[string]any{
			"nom":           cliNom.String,
			"prenom":        cliPrenom.String,
			"dateNaissance": "",
			"lieuNaissance": cliLieu.String,
			"typePiece":     cliPiece.String,
			"numeroPiece":   cliNum.String,
		}
		if cliNaissance.Valid {
			client["dateNaissance"] = cliNaissance.Time.Format("2006-01-02")
		}
		out["client"] = client
	}
	return out, nil
}

// sansClient retire le sous-objet client d'un DTO. Les trois endpoints de
// confirmation — la file, son détail et le POST — sont les seuls à ne pas le
// porter ; c'est mesuré sur quatre captures du 2026-08-27, pas déduit.
func sansClient(dto map[string]any) map[string]any {
	delete(dto, "client")
	return dto
}

// etatNumero lit l'état courant d'un numéro dans le registre et calcule
// RequestInProgress par existence d'une demande EN_COURS qui le référence. Un
// numéro absent du registre ne peut pas appartenir à l'opérateur source déclaré.
func (d *Deps) etatNumero(ctx context.Context, msisdn string) (entity.NumberState, *entity.Fault) {
	n := entity.NumberState{MSISDN: msisdn}

	err := d.DB.Pool.QueryRow(ctx,
		`SELECT operateur_actuel_id, operateur_origine_id, date_dernier_portage, deja_restitue
		   FROM numero WHERE msisdn = $1`, msisdn).
		Scan(&n.CurrentOperatorID, &n.OriginOperatorID, &n.LastPortingDate, &n.AlreadyRestituted)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.NumberState{}, entity.IncorrectSourceOperator()
	}
	if err != nil {
		return entity.NumberState{}, entity.InternalError("lecture du numéro")
	}

	if err := d.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM demande_numero dn
		    JOIN demande dm ON dm.id = dn.demande_id
		   WHERE dn.numero = $1
		     AND dm.statut_demande = 'EN_COURS'
		     AND NOT dn.exclu
		     AND dn.statut <> 'REJETE')`, msisdn).
		Scan(&n.RequestInProgress); err != nil {
		return entity.NumberState{}, entity.InternalError("lecture des demandes en cours")
	}

	return n, nil
}
