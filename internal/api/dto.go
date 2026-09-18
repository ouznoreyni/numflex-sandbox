package api

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/apperr"
	"github.com/ouznoreyni/numflex-sandbox/internal/domain"
)

// demandeDTO sérialise une demande au format du guide §7.3, commun à tous les
// endpoints qui renvoient une demande (Tasks 10 à 17). Tous les horodatages
// passent par d.R.Horodatage() : la dérive n'existe qu'au rendu, jamais en
// base, et la précision dépend de qui a écrit la valeur (paquet horodatage).
//
// Une flotte (ENTREPRISE) suit la même forme, à deux différences mesurées le
// 2026-09-18 contre rec-numflex.artp.sn : numeros[] remplace numero, et le
// client porte raisonSociale et numRC sans lieuNaissance.
func (d *Deps) demandeDTO(ctx context.Context, id string) (map[string]any, error) {
	var (
		numero, typeAbonne, typeDemande, statutDemande string
		etapeActuelle, statutEtapeActuel               string
		srcID, srcNom, dstID, dstNom                   string
		dateDemande                                    time.Time
		processus, routageInfo                         sql.NullString
		dateFinalisation                               sql.NullTime
		cliNom, cliPrenom, cliLieu, cliPiece, cliNum   sql.NullString
		cliRaisonSociale, cliNumRC                     sql.NullString
		cliNaissance                                   sql.NullTime
	)

	err := d.DB.Pool.QueryRow(ctx, `
		SELECT dem.numero, dem.type_abonne, dem.type_demande, dem.statut_demande,
		       dem.etape_actuelle, dem.statut_etape_actuel,
		       src.id, src.nom, dst.id, dst.nom,
		       dem.date_demande, dem.processus, dem.routage_info, dem.date_finalisation,
		       cli.nom, cli.prenom, cli.date_naissance, cli.lieu_naissance,
		       cli.type_piece, cli.numero_piece, cli.raison_sociale, cli.num_rc
		  FROM demande dem
		  JOIN operateur src ON src.id = dem.operateur_source_id
		  JOIN operateur dst ON dst.id = dem.operateur_destinataire_id
		  LEFT JOIN demande_client cli ON cli.demande_id = dem.id
		 WHERE dem.id = $1`, id).Scan(
		&numero, &typeAbonne, &typeDemande, &statutDemande,
		&etapeActuelle, &statutEtapeActuel,
		&srcID, &srcNom, &dstID, &dstNom,
		&dateDemande, &processus, &routageInfo, &dateFinalisation,
		&cliNom, &cliPrenom, &cliNaissance, &cliLieu, &cliPiece, &cliNum,
		&cliRaisonSociale, &cliNumRC)
	if err != nil {
		return nil, err
	}
	flotte := typeAbonne == string(domain.AbonneEntreprise)

	out := map[string]any{
		"id":                    id,
		"typeAbonne":            typeAbonne,
		"typeDemande":           typeDemande,
		"statutDemande":         statutDemande,
		"etapeActuelle":         etapeActuelle,
		"statutEtapeActuel":     statutEtapeActuel,
		"operateurSource":       map[string]any{"id": srcID, "nom": srcNom},
		"operateurDestinataire": map[string]any{"id": dstID, "nom": dstNom},
		"dateDemande":           d.R.Horodatage(ctx, dateDemande),
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
		out["dateFinalisation"] = d.R.Horodatage(ctx, dateFinalisation.Time)
	}

	// Un particulier porte son numéro ; une flotte porte la liste des numéros
	// retenus, dans l'ordre déclaré, et jamais le porteur seul (demande.numero
	// reste interne : c'est le numéro qui a reçu l'OTP).
	if flotte {
		numeros, err := d.numerosFlotte(ctx, id)
		if err != nil {
			return nil, err
		}
		out["numeros"] = numeros
	} else {
		out["numero"] = numero
	}

	// Le client est rendu dans toutes les captures — création, acceptation,
	// traitement, a-traiter, in. Il est absent des seules réponses de
	// confirmation, que sansClient dépouille. Particulier : six champs,
	// lieuNaissance compris (2026-08-27). Flotte : sept champs, raisonSociale
	// et numRC à la place de lieuNaissance (2026-09-18).
	if cliNom.Valid || cliPrenom.Valid || cliNum.Valid {
		client := map[string]any{
			"nom":           cliNom.String,
			"prenom":        cliPrenom.String,
			"dateNaissance": "",
			"typePiece":     cliPiece.String,
			"numeroPiece":   cliNum.String,
		}
		if cliNaissance.Valid {
			client["dateNaissance"] = cliNaissance.Time.Format("2006-01-02")
		}
		if flotte {
			client["raisonSociale"] = cliRaisonSociale.String
			client["numRC"] = cliNumRC.String
		} else {
			client["lieuNaissance"] = cliLieu.String
		}
		out["client"] = client
	}
	return out, nil
}

// numerosFlotte liste les numéros d'une flotte qui n'ont pas été exclus à la
// création, dans l'ordre où la demande les a déclarés. Un numéro rejeté à
// l'acceptation y reste : il fait partie de la demande, même s'il ne sera pas
// porté.
func (d *Deps) numerosFlotte(ctx context.Context, id string) ([]string, error) {
	rows, err := d.DB.Pool.Query(ctx,
		`SELECT numero FROM demande_numero
		  WHERE demande_id = $1 AND NOT exclu
		  ORDER BY position, numero`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	numeros := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		numeros = append(numeros, n)
	}
	return numeros, rows.Err()
}

// sansClient retire le sous-objet client d'un DTO. Les trois endpoints de
// confirmation — la file, son détail et le POST — sont les seuls à ne pas le
// porter ; c'est mesuré sur quatre captures du 2026-08-27, pas déduit.
func sansClient(dto map[string]any) map[string]any {
	delete(dto, "client")
	return dto
}

// etatNumero lit l'état courant d'un numéro dans le registre et calcule
// DemandeEnCours par existence d'une demande EN_COURS qui le référence. Un
// numéro absent du registre ne peut pas appartenir à l'opérateur source déclaré.
func (d *Deps) etatNumero(ctx context.Context, msisdn string) (domain.EtatNumero, *apperr.Error) {
	n := domain.EtatNumero{MSISDN: msisdn}

	err := d.DB.Pool.QueryRow(ctx,
		`SELECT operateur_actuel_id, operateur_origine_id, date_dernier_portage, deja_restitue
		   FROM numero WHERE msisdn = $1`, msisdn).
		Scan(&n.OperateurActuelID, &n.OperateurOrigineID, &n.DateDernierPortage, &n.DejaRestitue)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EtatNumero{}, apperr.OperateurSourceIncorrect()
	}
	if err != nil {
		return domain.EtatNumero{}, apperr.ErreurInterne("lecture du numéro")
	}

	if err := d.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM demande_numero dn
		    JOIN demande dm ON dm.id = dn.demande_id
		   WHERE dn.numero = $1
		     AND dm.statut_demande = 'EN_COURS'
		     AND NOT dn.exclu
		     AND dn.statut <> 'REJETE')`, msisdn).
		Scan(&n.DemandeEnCours); err != nil {
		return domain.EtatNumero{}, apperr.ErreurInterne("lecture des demandes en cours")
	}

	return n, nil
}
