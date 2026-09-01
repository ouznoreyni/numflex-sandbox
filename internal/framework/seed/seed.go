// Package seed populates a fresh database with the operators, rejection
// reasons, request types, processes, incident types, accounts and number
// pool the sandbox needs from its first startup. Run is idempotent — every
// statement is an INSERT ... ON CONFLICT DO NOTHING.
package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"golang.org/x/crypto/bcrypt"
)

// Identifiers recorded at ARTP SIT — MongoDB ObjectId, not to be changed.
const (
	OperateurOrange   = "6a21745ce6c37b5b5b487ec1"
	OperateurYAS      = "6a2174c3e6c37b5b5b487ec4"
	OperateurExpresso = "6a217510e6c37b5b5b487ec7"

	MotifDernierPortage3Mois = "6a2175c5e6c37b5b5b487edb"
	MotifErreurInfos         = "6a2175cfe6c37b5b5b487edc"
	MotifDonneesManquantes   = "6a2175d9e6c37b5b5b487edd"
	MotifNumeroInactif       = "6a2175e7e6c37b5b5b487ede"
	MotifIdentiteNonProuvee  = "6a2175f3e6c37b5b5b487edf"
	MotifEngagementEnCours   = "6a2175fde6c37b5b5b487ee0"

	TypeDemandePortage     = "6a217518e6c37b5b5b487ec8"
	TypeDemandeRestitution = "6a21751be6c37b5b5b487ec9"
	TypeDemandeReverse     = "6a21751fe6c37b5b5b487eca"

	ProcessusPrepaid  = "6a217686e6c37b5b5b487ee8"
	ProcessusPostpaid = "6a217689e6c37b5b5b487ee9"

	// Identifiers from guide v2 §7.1 — the only published values.
	TypeIncidentGateway   = "65abc456def001"
	TypeIncidentTechnique = "65abc456def002"
)

func Run(ctx context.Context, db *persistence.DB) error {
	operateurs := []struct{ id, nom, prefixe string }{
		{OperateurOrange, "ORANGE", "191"},
		{OperateurYAS, "YAS", "192"},
		// [HYP] EXPRESSO's routing prefix was not observed at SIT; 191 and 192 were.
		{OperateurExpresso, "EXPRESSO", "193"},
	}
	for _, o := range operateurs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO operateur (id, nom, prefixe_routage) VALUES ($1,$2,$3)
			 ON CONFLICT (id) DO NOTHING`, o.id, o.nom, o.prefixe); err != nil {
			return fmt.Errorf("seed opérateur %s : %w", o.nom, err)
		}
	}

	motifs := []struct{ id, motif string }{
		{MotifDernierPortage3Mois, "Dernier portage inférieur à 3 mois"},
		{MotifErreurInfos, "Erreur sur les infos"},
		{MotifDonneesManquantes, "Données manquantes"},
		{MotifNumeroInactif, "Numéro Inactif"},
		{MotifIdentiteNonProuvee, "Identité non prouvée"},
		{MotifEngagementEnCours, "Engagement en cours dans une demande"},
	}
	for _, m := range motifs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO motif_rejet (id, motif) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, m.id, m.motif); err != nil {
			return fmt.Errorf("seed motif : %w", err)
		}
	}

	types := []struct{ id, t string }{
		{TypeDemandePortage, "PORTAGE"},
		{TypeDemandeRestitution, "RESTITUTION"},
		{TypeDemandeReverse, "REVERSE"},
	}
	for _, x := range types {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO type_demande (id, type) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, x.id, x.t); err != nil {
			return fmt.Errorf("seed type de demande : %w", err)
		}
	}

	procs := []struct{ id, t string }{
		{ProcessusPrepaid, "PREPAID"},
		{ProcessusPostpaid, "POSTPAID"},
	}
	for _, p := range procs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO processus (id, type) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, p.id, p.t); err != nil {
			return fmt.Errorf("seed processus : %w", err)
		}
	}

	incidents := []struct {
		id, libelle string
		fige        bool
	}{
		{TypeIncidentGateway, "Gateway", false},
		{TypeIncidentTechnique, "Technique", true},
	}
	for _, i := range incidents {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO type_incident (id, libelle, fige_systeme) VALUES ($1,$2,$3)
			 ON CONFLICT (id) DO NOTHING`, i.id, i.libelle, i.fige); err != nil {
			return fmt.Errorf("seed type d'incident : %w", err)
		}
	}

	comptes := []struct{ username, motDePasse, operateur string }{
		{"orange", "orange2026", OperateurOrange},
		{"yas", "yas2026", OperateurYAS},
		{"expresso", "expresso2026", OperateurExpresso},
	}
	for _, c := range comptes {
		hash, err := bcrypt.GenerateFromPassword([]byte(c.motDePasse), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hachage %s : %w", c.username, err)
		}
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO utilisateur (id, username, password_hash, operateur_id, roles)
			 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (username) DO NOTHING`,
			c.operateur+"-user", c.username, string(hash), c.operateur,
			[]string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"}); err != nil {
			return fmt.Errorf("seed compte %s : %w", c.username, err)
		}
	}

	return seedNumbers(ctx, db)
}

// seedNumbers installs the pool described in spec §10: ten numbers per
// range, each range making one precise rule exercisable from the first
// startup.
func seedNumbers(ctx context.Context, db *persistence.DB) error {
	jours := func(n int) *time.Time {
		t := time.Now().AddDate(0, 0, -n)
		return &t
	}

	tranches := []struct {
		prefixe  string
		actuel   string
		origine  string
		portage  *time.Time
		restitue bool
	}{
		{"77100", OperateurOrange, OperateurOrange, nil, false},
		{"76100", OperateurYAS, OperateurYAS, nil, false},
		{"70100", OperateurExpresso, OperateurExpresso, nil, false},
		{"77200", OperateurOrange, OperateurYAS, jours(30), false},
		{"77300", OperateurYAS, OperateurOrange, jours(240), false},
		{"77400", OperateurYAS, OperateurOrange, jours(60), false},
		{"77500", OperateurYAS, OperateurOrange, jours(240), true},
	}

	for _, tr := range tranches {
		for i := 1; i <= 10; i++ {
			msisdn := fmt.Sprintf("%s%04d", tr.prefixe, i)
			if _, err := db.Pool.Exec(ctx,
				`INSERT INTO numero
				   (msisdn, operateur_actuel_id, operateur_origine_id,
				    date_dernier_portage, deja_restitue, actif)
				 VALUES ($1,$2,$3,$4,$5,true)
				 ON CONFLICT (msisdn) DO NOTHING`,
				msisdn, tr.actuel, tr.origine, tr.portage, tr.restitue); err != nil {
				return fmt.Errorf("seed numéro %s : %w", msisdn, err)
			}
		}
	}
	return nil
}
