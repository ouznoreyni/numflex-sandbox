package api

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
)

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
	)

	err := d.DB.Pool.QueryRow(ctx, `
		SELECT dem.numero, dem.type_abonne, dem.type_demande, dem.statut_demande,
		       dem.etape_actuelle, dem.statut_etape_actuel,
		       src.id, src.nom, dst.id, dst.nom,
		       dem.date_demande, dem.processus, dem.routage_info, dem.date_finalisation
		  FROM demande dem
		  JOIN operateur src ON src.id = dem.operateur_source_id
		  JOIN operateur dst ON dst.id = dem.operateur_destinataire_id
		 WHERE dem.id = $1`, id).Scan(
		&numero, &typeAbonne, &typeDemande, &statutDemande,
		&etapeActuelle, &statutEtapeActuel,
		&srcID, &srcNom, &dstID, &dstNom,
		&dateDemande, &processus, &routageInfo, &dateFinalisation)
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
	return out, nil
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
