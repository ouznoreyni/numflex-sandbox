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

func Run(ctx context.Context, db *persistence.DB, v Volumes) error {
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

	return seedNumbers(ctx, db, v)
}

// UnportedRangesPerOperator is how many never-ported ranges each operator
// owns — the eight of the 100 to 800 groups. The pool a porting test
// consumes is spread evenly over them.
const UnportedRangesPerOperator = 8

// MaxPerRange is what a range holds: the tail is six digits, so the million
// numbers running from 000000 to 999999 — 771 is 771000000 to 771999999,
// whole. lpad truncates on the right beyond that, which would silently fold
// 1000000 onto 100000 and lose the number to ON CONFLICT.
const MaxPerRange = 1_000_000

// Volumes fixes how many numbers each kind of range carries. It exists
// because the pool is seeded again at every startup and, in the test suite,
// at every test: what makes a usable sandbox — millions of numbers — would
// make an unusable suite.
type Volumes struct {
	// OrangeYAS is what each of the eight never-ported ranges of ORANGE and
	// YAS carries. This is the pool that a successful porting consumes, one
	// number at a time.
	OrangeYAS int
	// Expresso is the same for the third operator, present so that a porting
	// between two third parties (UC-08) is exercisable — not to be consumed
	// in volume.
	Expresso int
	// Historical is for the two ranges 76100 and 70100, kept so numbers
	// published before the pool was widened stay valid.
	Historical int
	// PortedBlock is what each scenario block of the 900 group carries.
	// Rejection material: a thousand per case is plenty.
	PortedBlock int
}

// fixedRangeSize is what every range carries that is rejection material
// rather than portable stock: EXPRESSO's, the two historical ones, and each
// block of the 900 group. A thousand per case is plenty, and costs nothing.
const fixedRangeSize = 1000

// TestVolumes is what the test suite seeds: enough for every number
// published in the README to exist, small enough to reseed between tests.
var TestVolumes = Volumes{
	OrangeYAS:   fixedRangeSize,
	Expresso:    fixedRangeSize,
	Historical:  fixedRangeSize,
	PortedBlock: fixedRangeSize,
}

// VolumesFor spreads perOperator numbers over the eight never-ported ranges
// of ORANGE and YAS, leaving every other range at fixedRangeSize.
func VolumesFor(perOperator int) Volumes {
	return Volumes{
		OrangeYAS:   perOperator / UnportedRangesPerOperator,
		Expresso:    fixedRangeSize,
		Historical:  fixedRangeSize,
		PortedBlock: fixedRangeSize,
	}
}

// unportedRanges — the never-ported pool, eight ranges per operator. The
// prefix is three digits and the tail six, so a number keeps the identity it
// had when ranges were five digits and tails four: 771 + 000001 is the same
// 771000001 as 77100 + 0001.
var unportedRanges = []struct{ prefix, operator string }{
	{"771", OperatorOrangeID}, {"772", OperatorOrangeID},
	{"773", OperatorOrangeID}, {"774", OperatorOrangeID},
	{"775", OperatorOrangeID}, {"776", OperatorOrangeID},
	{"777", OperatorOrangeID}, {"778", OperatorOrangeID},

	{"781", OperatorYASID}, {"782", OperatorYASID},
	{"783", OperatorYASID}, {"784", OperatorYASID},
	{"785", OperatorYASID}, {"786", OperatorYASID},
	{"787", OperatorYASID}, {"788", OperatorYASID},

	{"711", OperatorExpressoID}, {"712", OperatorExpressoID},
	{"713", OperatorExpressoID}, {"714", OperatorExpressoID},
	{"715", OperatorExpressoID}, {"716", OperatorExpressoID},
	{"717", OperatorExpressoID}, {"718", OperatorExpressoID},
}

// historicalRanges — 76100 and 70100 under their new three-digit reading.
var historicalRanges = []struct{ prefix, operator string }{
	{"761", OperatorYASID},
	{"701", OperatorExpressoID},
}

// portedRanges — the 900 group, one range per operator, where every number
// has already been ported. Each range stacks the four scenarios in
// consecutive blocks, in the order of portedScenarios.
var portedRanges = []struct{ prefix, current, origin string }{
	{"779", OperatorOrangeID, OperatorYASID},
	{"789", OperatorYASID, OperatorOrangeID},
	{"719", OperatorExpressoID, OperatorOrangeID},
}

// portedScenarios — the four situations a ported number can be in, each
// making one rule exercisable from the first startup. Block n of a ported
// range holds scenario n.
var portedScenarios = [...]struct {
	daysAgo  int
	returned bool
}{
	{30, false},  // DELAI_PORTAGE_NON_RESPECTE — ported less than 3 months ago
	{240, false}, // nominal restitution — ported more than 6 months ago
	{60, false},  // DELAI_RESTITUTION_NON_RESPECTE — ported 2 months ago
	{240, true},  // NUMERO_DEJA_RESTITUE — ported, then already restituted
}

// PortedScenarioCount is how many scenario blocks a ported range stacks.
const PortedScenarioCount = len(portedScenarios)

// seedNumbers installs the pool, one statement per range rather than one per
// number: at two million numbers per operator the unitary INSERT would cost
// as many round trips at every startup.
func seedNumbers(ctx context.Context, db *persistence.DB, v Volumes) error {
	for _, r := range unportedRanges {
		size := v.OrangeYAS
		if r.operator == OperatorExpressoID {
			size = v.Expresso
		}
		if err := insertRange(ctx, db, r.prefix, r.operator, r.operator,
			nil, false, 0, size); err != nil {
			return err
		}
	}

	for _, r := range historicalRanges {
		if err := insertRange(ctx, db, r.prefix, r.operator, r.operator,
			nil, false, 0, v.Historical); err != nil {
			return err
		}
	}

	for _, r := range portedRanges {
		for n, sc := range portedScenarios {
			date := time.Now().AddDate(0, 0, -sc.daysAgo)
			if err := insertRange(ctx, db, r.prefix, r.current, r.origin,
				&date, sc.returned, n*v.PortedBlock, v.PortedBlock); err != nil {
				return err
			}
		}
	}
	return nil
}

// insertRange inserts size numbers whose six-digit tails run from first, all
// sharing one situation. A range starts at 000000: a full one runs from
// first=0 to the millionth number, 999999.
//
// It returns early when the last number of the range is already there. A
// range is installed by a single statement, hence whole or not at all, so
// that one number answers for all of them — and the shortcut is what keeps a
// restart on persistent data instant: letting two million rows conflict away
// one by one costs as much as inserting them.
func insertRange(ctx context.Context, db *persistence.DB, prefix,
	current, origin string, porting *time.Time, returned bool,
	first, size int) error {

	if size <= 0 {
		return nil
	}
	if first+size > MaxPerRange {
		return fmt.Errorf("tranche %s : %d numéros dépassent la capacité %d",
			prefix, first+size, MaxPerRange)
	}
	last := first + size - 1

	var present bool
	if err := db.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM numero WHERE msisdn = $1)`,
		fmt.Sprintf("%s%06d", prefix, last)).Scan(&present); err != nil {
		return fmt.Errorf("seed tranche %s : %w", prefix, err)
	}
	if present {
		return nil
	}

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO numero
		   (msisdn, operateur_actuel_id, operateur_origine_id,
		    date_dernier_portage, deja_restitue, actif)
		 SELECT $1 || lpad(g::text, 6, '0'), $2, $3,
		        $4::timestamptz, $5::boolean, true
		 FROM generate_series($6::int, $7::int) AS g
		 ON CONFLICT (msisdn) DO NOTHING`,
		prefix, current, origin, porting, returned,
		first, last); err != nil {
		return fmt.Errorf("seed tranche %s à partir de %06d : %w", prefix, first, err)
	}
	return nil
}
