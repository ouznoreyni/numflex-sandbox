//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
)

// orangeID is seed.OperatorOrangeID, spelled out: an adapter test may not
// import the framework layer.
const orangeID = "6a21745ce6c37b5b5b487ec1"

// homes is the part of seed.HomeRanges these tests exercise.
var homes = map[string]string{"771": orangeID}

// TestNumberGatewayOpensHomeRanges pins the open registry: a well-formed
// number of a home range exists even though the seed never wrote it, held
// by the range's operator and never ported — and it is written on that
// first read, so the porting that follows finds a row to transfer.
func TestNumberGatewayOpensHomeRanges(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewNumberGateway(db.Pool, homes)
	ctx := context.Background()

	// seed.TestVolumes stops each range at 000999: 771987654 is not seeded.
	const msisdn = "771987654"

	n, found, err := g.State(ctx, msisdn)
	must(t, err)
	if !found {
		t.Fatal("a number of a home range must exist without being seeded")
	}
	if n.CurrentOperatorID != orangeID || n.OriginOperatorID != orangeID {
		t.Fatalf("771987654 should be ORANGE's, at home: got %+v", n)
	}
	if n.LastPortingDate != nil || n.AlreadyRestituted || n.RequestInProgress {
		t.Fatalf("an unseeded number must start never ported: got %+v", n)
	}

	var rows int
	must(t, db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM numero WHERE msisdn = $1`, msisdn).Scan(&rows))
	if rows != 1 {
		t.Fatalf("the first read must write the number: %d rows", rows)
	}
}

// TestNumberGatewayKeepsOtherNumbersClosed: the registry opens on home
// ranges only. A prefix no operator owns does not exist, and a malformed
// number is not a number.
func TestNumberGatewayKeepsOtherNumbersClosed(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewNumberGateway(db.Pool, homes)
	ctx := context.Background()

	for _, msisdn := range []string{
		"791000001",  // no operator's range
		"77100000a",  // not digits
		"7710000001", // ten digits
	} {
		_, found, err := g.State(ctx, msisdn)
		must(t, err)
		if found {
			t.Errorf("%s must not exist", msisdn)
		}
	}
}

// TestNumberGatewayKeepsAHistory: opening a range never rewrites a number
// already there. 789001001 is seeded ported from ORANGE eight months ago;
// with 789 open under YAS, it is still that number.
func TestNumberGatewayKeepsAHistory(t *testing.T) {
	db := testsupport.NewTestDB(t)
	const yasID = "6a2174c3e6c37b5b5b487ec4"
	g := postgres.NewNumberGateway(db.Pool, map[string]string{"789": yasID})

	n, found, err := g.State(context.Background(), "789001001")
	must(t, err)
	if !found || n.LastPortingDate == nil || n.OriginOperatorID != orangeID {
		t.Fatalf("789001001 must keep its seeded porting: got %+v", n)
	}
}
