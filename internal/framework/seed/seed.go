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
	OperatorOrangeID   = "6a21745ce6c37b5b5b487ec1"
	OperatorYASID      = "6a2174c3e6c37b5b5b487ec4"
	OperatorExpressoID = "6a217510e6c37b5b5b487ec7"

	RejectionReasonLastPorting3MonthsID = "6a2175c5e6c37b5b5b487edb"
	RejectionReasonWrongInfoID          = "6a2175cfe6c37b5b5b487edc"
	RejectionReasonMissingDataID        = "6a2175d9e6c37b5b5b487edd"
	RejectionReasonInactiveNumberID     = "6a2175e7e6c37b5b5b487ede"
	RejectionReasonIdentityNotProvenID  = "6a2175f3e6c37b5b5b487edf"
	RejectionReasonOngoingCommitmentID  = "6a2175fde6c37b5b5b487ee0"

	RequestTypePortingID     = "6a217518e6c37b5b5b487ec8"
	RequestTypeRestitutionID = "6a21751be6c37b5b5b487ec9"
	RequestTypeReverseID     = "6a21751fe6c37b5b5b487eca"

	ProcessPrepaidID  = "6a217686e6c37b5b5b487ee8"
	ProcessPostpaidID = "6a217689e6c37b5b5b487ee9"

	// Identifiers from guide v2 §7.1 — the only published values.
	IncidentTypeGatewayID   = "65abc456def001"
	IncidentTypeTechnicalID = "65abc456def002"
)

func Run(ctx context.Context, db *persistence.DB) error {
	operators := []struct{ id, name, prefix string }{
		{OperatorOrangeID, "ORANGE", "191"},
		{OperatorYASID, "YAS", "192"},
		// [HYP] EXPRESSO's routing prefix was not observed at SIT; 191 and 192 were.
		{OperatorExpressoID, "EXPRESSO", "193"},
	}
	for _, o := range operators {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO operateur (id, nom, prefixe_routage) VALUES ($1,$2,$3)
			 ON CONFLICT (id) DO NOTHING`, o.id, o.name, o.prefix); err != nil {
			return fmt.Errorf("seed opérateur %s : %w", o.name, err)
		}
	}

	reasons := []struct{ id, reason string }{
		{RejectionReasonLastPorting3MonthsID, "Dernier portage inférieur à 3 mois"},
		{RejectionReasonWrongInfoID, "Erreur sur les infos"},
		{RejectionReasonMissingDataID, "Données manquantes"},
		{RejectionReasonInactiveNumberID, "Numéro Inactif"},
		{RejectionReasonIdentityNotProvenID, "Identité non prouvée"},
		{RejectionReasonOngoingCommitmentID, "Engagement en cours dans une demande"},
	}
	for _, r := range reasons {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO motif_rejet (id, motif) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, r.id, r.reason); err != nil {
			return fmt.Errorf("seed motif : %w", err)
		}
	}

	types := []struct{ id, t string }{
		{RequestTypePortingID, "PORTAGE"},
		{RequestTypeRestitutionID, "RESTITUTION"},
		{RequestTypeReverseID, "REVERSE"},
	}
	for _, x := range types {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO type_demande (id, type) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, x.id, x.t); err != nil {
			return fmt.Errorf("seed type de demande : %w", err)
		}
	}

	procs := []struct{ id, t string }{
		{ProcessPrepaidID, "PREPAID"},
		{ProcessPostpaidID, "POSTPAID"},
	}
	for _, p := range procs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO processus (id, type) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, p.id, p.t); err != nil {
			return fmt.Errorf("seed processus : %w", err)
		}
	}

	incidents := []struct {
		id, label string
		frozen    bool
	}{
		{IncidentTypeGatewayID, "Gateway", false},
		{IncidentTypeTechnicalID, "Technique", true},
	}
	for _, i := range incidents {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO type_incident (id, libelle, fige_systeme) VALUES ($1,$2,$3)
			 ON CONFLICT (id) DO NOTHING`, i.id, i.label, i.frozen); err != nil {
			return fmt.Errorf("seed type d'incident : %w", err)
		}
	}

	accounts := []struct{ username, password, operator string }{
		{"orange", "orange2026", OperatorOrangeID},
		{"yas", "yas2026", OperatorYASID},
		{"expresso", "expresso2026", OperatorExpressoID},
	}
	for _, a := range accounts {
		hash, err := bcrypt.GenerateFromPassword([]byte(a.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hachage %s : %w", a.username, err)
		}
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO utilisateur (id, username, password_hash, operateur_id, roles)
			 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (username) DO NOTHING`,
			a.operator+"-user", a.username, string(hash), a.operator,
			[]string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"}); err != nil {
			return fmt.Errorf("seed compte %s : %w", a.username, err)
		}
	}

	return seedNumbers(ctx, db)
}

// seedNumbers installs the pool described in spec §10: ten numbers per
// range, each range making one precise rule exercisable from the first
// startup.
func seedNumbers(ctx context.Context, db *persistence.DB) error {
	daysAgo := func(n int) *time.Time {
		t := time.Now().AddDate(0, 0, -n)
		return &t
	}

	ranges := []struct {
		prefix   string
		current  string
		origin   string
		porting  *time.Time
		returned bool
	}{
		{"77100", OperatorOrangeID, OperatorOrangeID, nil, false},
		{"76100", OperatorYASID, OperatorYASID, nil, false},
		{"70100", OperatorExpressoID, OperatorExpressoID, nil, false},
		{"77200", OperatorOrangeID, OperatorYASID, daysAgo(30), false},
		{"77300", OperatorYASID, OperatorOrangeID, daysAgo(240), false},
		{"77400", OperatorYASID, OperatorOrangeID, daysAgo(60), false},
		{"77500", OperatorYASID, OperatorOrangeID, daysAgo(240), true},
	}

	for _, r := range ranges {
		for i := 1; i <= 10; i++ {
			msisdn := fmt.Sprintf("%s%04d", r.prefix, i)
			if _, err := db.Pool.Exec(ctx,
				`INSERT INTO numero
				   (msisdn, operateur_actuel_id, operateur_origine_id,
				    date_dernier_portage, deja_restitue, actif)
				 VALUES ($1,$2,$3,$4,$5,true)
				 ON CONFLICT (msisdn) DO NOTHING`,
				msisdn, r.current, r.origin, r.porting, r.returned); err != nil {
				return fmt.Errorf("seed numéro %s : %w", msisdn, err)
			}
		}
	}
	return nil
}
