package postgres

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// NumberRangeGateway is the Postgres implementation of
// port.NumberRangeGateway: a single aggregate over the numero table.
type NumberRangeGateway struct {
	db Querier
}

// NewNumberRangeGateway returns a gateway bound to db — the pool, in
// practice: the one query below reads, and reads nothing another statement
// of the same operation depends on.
func NewNumberRangeGateway(db Querier) *NumberRangeGateway {
	return &NumberRangeGateway{db: db}
}

// RangesByOperator counts the operator's numbers per three-digit prefix.
//
// The grouping is on the rows rather than on the ranges internal/framework/
// seed declares, so that the answer stays true to the database: a range
// installed at another volume reports its real size, and a number that has
// changed hands is counted under its new holder.
//
// It costs what counting costs. Measured on the full pool — eight million
// numbers for the operator — this is a 2.7 s sequential scan (4.6 s on the
// very first call after startup, cold cache), and no index redeems it: a
// composite (operateur_actuel_id, msisdn) index was measured at 902 MB for
// no gain at all, the aggregate still having to visit every row. The route is introspection, not a step of the porting cycle, and a
// sandbox started at a smaller POOL_NUMBERS_PER_OPERATOR answers in
// milliseconds.
func (g *NumberRangeGateway) RangesByOperator(ctx context.Context, operatorID string) ([]port.NumberRange, error) {
	rows, err := g.db.Query(ctx,
		`SELECT left(msisdn, 3) AS prefixe,
		        count(*) AS total,
		        min(msisdn) AS premier,
		        max(msisdn) AS dernier,
		        count(*) FILTER (WHERE date_dernier_portage IS NOT NULL) AS portes
		 FROM numero
		 WHERE operateur_actuel_id = $1
		 GROUP BY 1
		 ORDER BY 1`, operatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []port.NumberRange{}
	for rows.Next() {
		var r port.NumberRange
		if err := rows.Scan(&r.Prefix, &r.Total, &r.First, &r.Last, &r.Ported); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	// Next() returning false means either "end of rows" or "error": without
	// this check, a failure mid-iteration would pass for a partial success.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
