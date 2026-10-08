package testsupport

import (
	"context"
	"fmt"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
)

// The number fixtures the test suite starts from. The server seeds none —
// its registry fills from the requests it receives — but a test needs
// numbers already in a given situation: at home, ported a month ago, ported
// eight months ago, already restituted. They live here, beside the suite
// that uses them, and nowhere the server can reach.

// RangeSize is what every fixture range carries: a range starts at 000000,
// so a thousand-strong one stops at 000999.
const RangeSize = 1000

// UnportedRangesPerOperator is how many never-ported ranges each operator
// holds in the fixtures.
const UnportedRangesPerOperator = 8

// unportedRanges — never-ported numbers, eight ranges per operator.
var unportedRanges = []struct{ prefix, operator string }{
	{"771", seed.OperatorOrangeID}, {"772", seed.OperatorOrangeID},
	{"773", seed.OperatorOrangeID}, {"774", seed.OperatorOrangeID},
	{"775", seed.OperatorOrangeID}, {"776", seed.OperatorOrangeID},
	{"777", seed.OperatorOrangeID}, {"778", seed.OperatorOrangeID},

	{"781", seed.OperatorYASID}, {"782", seed.OperatorYASID},
	{"783", seed.OperatorYASID}, {"784", seed.OperatorYASID},
	{"785", seed.OperatorYASID}, {"786", seed.OperatorYASID},
	{"787", seed.OperatorYASID}, {"788", seed.OperatorYASID},

	{"711", seed.OperatorExpressoID}, {"712", seed.OperatorExpressoID},
	{"713", seed.OperatorExpressoID}, {"714", seed.OperatorExpressoID},
	{"715", seed.OperatorExpressoID}, {"716", seed.OperatorExpressoID},
	{"717", seed.OperatorExpressoID}, {"718", seed.OperatorExpressoID},

	{"761", seed.OperatorYASID},
	{"701", seed.OperatorExpressoID},
}

// portedRanges — one range per operator where every number has already
// been ported. Each stacks the scenarios of portedScenarios in consecutive
// blocks of RangeSize.
var portedRanges = []struct{ prefix, current, origin string }{
	{"779", seed.OperatorOrangeID, seed.OperatorYASID},
	{"789", seed.OperatorYASID, seed.OperatorOrangeID},
	{"719", seed.OperatorExpressoID, seed.OperatorOrangeID},
}

// portedScenarios — block n of a ported range holds scenario n.
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

// SeedNumbers writes the fixtures, one statement per range.
func SeedNumbers(ctx context.Context, db *persistence.DB) error {
	for _, r := range unportedRanges {
		if err := insertRange(ctx, db, r.prefix, r.operator, r.operator,
			nil, false, 0); err != nil {
			return err
		}
	}
	for _, r := range portedRanges {
		for n, sc := range portedScenarios {
			date := time.Now().AddDate(0, 0, -sc.daysAgo)
			if err := insertRange(ctx, db, r.prefix, r.current, r.origin,
				&date, sc.returned, n*RangeSize); err != nil {
				return err
			}
		}
	}
	return nil
}

// insertRange inserts RangeSize numbers whose six-digit tails start at
// first, all sharing one situation.
func insertRange(ctx context.Context, db *persistence.DB, prefix,
	current, origin string, porting *time.Time, returned bool, first int) error {

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO numero
		   (msisdn, operateur_actuel_id, operateur_origine_id,
		    date_dernier_portage, deja_restitue, actif)
		 SELECT $1 || lpad(g::text, 6, '0'), $2, $3,
		        $4::timestamptz, $5::boolean, true
		 FROM generate_series($6::int, $7::int) AS g
		 ON CONFLICT (msisdn) DO NOTHING`,
		prefix, current, origin, porting, returned,
		first, first+RangeSize-1); err != nil {
		return fmt.Errorf("fixture tranche %s à partir de %06d : %w", prefix, first, err)
	}
	return nil
}
