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

const yasID = "6a2174c3e6c37b5b5b487ec4"

// TestNumberGatewayHasNoHardCodedRange: a number the registry does not
// hold does not exist, whatever its prefix — 768 included, which no fixture
// writes — and reading it writes nothing.
func TestNumberGatewayHasNoHardCodedRange(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewNumberGateway(db.Pool)
	ctx := context.Background()

	for _, msisdn := range []string{"768012042", "771987654", "791000001"} {
		_, found, err := g.State(ctx, msisdn)
		must(t, err)
		if found {
			t.Errorf("%s is not in the registry and must not exist", msisdn)
		}
	}

	var rows int
	must(t, db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM numero WHERE msisdn = '768012042'`).Scan(&rows))
	if rows != 0 {
		t.Fatalf("a read must write nothing: %d rows", rows)
	}
}

// TestNumberGatewayRegisters: Register writes an unknown number at the
// operator given, at home and never ported.
func TestNumberGatewayRegisters(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewNumberGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Register(ctx, "768012042", yasID))

	n, found, err := g.State(ctx, "768012042")
	must(t, err)
	if !found {
		t.Fatal("a registered number must exist")
	}
	if n.CurrentOperatorID != yasID || n.OriginOperatorID != yasID {
		t.Fatalf("768012042 should be YAS's, at home: got %+v", n)
	}
	if n.LastPortingDate != nil || n.AlreadyRestituted || n.RequestInProgress {
		t.Fatalf("a registered number must start never ported: got %+v", n)
	}
}

// TestNumberGatewayRegisterKeepsAHistory: registering a number already
// there never rewrites it. 789001001 is a fixture ported from ORANGE to YAS
// eight months ago; registering it at ORANGE leaves it that number.
func TestNumberGatewayRegisterKeepsAHistory(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewNumberGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Register(ctx, "789001001", orangeID))

	n, found, err := g.State(ctx, "789001001")
	must(t, err)
	if !found || n.CurrentOperatorID != yasID || n.LastPortingDate == nil ||
		n.OriginOperatorID != orangeID {
		t.Fatalf("789001001 must keep its porting: got %+v", n)
	}
}

// TestNumberGatewayRegisterNeedsAnOperator: an operator the registry does
// not know writes nothing.
func TestNumberGatewayRegisterNeedsAnOperator(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewNumberGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Register(ctx, "768012042", "operateur-inconnu"))

	_, found, err := g.State(ctx, "768012042")
	must(t, err)
	if found {
		t.Fatal("an unknown operator must not register a number")
	}
}
