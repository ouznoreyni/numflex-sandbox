//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// seedFleetRequest inserts a bare ENTREPRISE request at ACCEPTATION, with
// two demande_numero rows, directly by SQL — internal/usecase/acceptance
// has no Create of its own to reuse (it starts from a request request
// creation already produced), so this is the same level of directness
// TestUnitOfWorkRollsBack (in unit_of_work_test.go) uses for OTP.
func seedFleetRequest(t *testing.T, db *persistence.DB, id string, numbers ...string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO demande
		  (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		   statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		   createur_operateur_id, date_demande, date_debut_etape)
		VALUES ($1, $2, 'ENTREPRISE', 'PORTAGE', 'EN_COURS', 'ACCEPTATION', 'EN_COURS',
		        $3, $4, $4, $5, $5)`,
		id, numbers[0], seed.OperatorOrangeID, seed.OperatorYASID, now)
	if err != nil {
		t.Fatalf("seed demande : %v", err)
	}

	for _, n := range numbers {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO demande_numero (demande_id, numero, statut) VALUES ($1, $2, 'EN_COURS')`,
			id, n); err != nil {
			t.Fatalf("seed demande_numero %s : %v", n, err)
		}
	}
}

// TestAcceptanceRejectionRollsBack proves the guarantee Task 14 exists to
// preserve: a fleet's number-by-number rejection writes RejectNumber calls
// and, once the fleet is exhausted, a Reject call — all inside one
// port.UnitOfWork.Do — and a failure anywhere in that closure leaves every
// one of those writes undone, RejectNumber's included even though it ran
// and returned successfully before the failure. Without this, "one
// transaction" is a claim rather than a guarantee — the same point
// TestUnitOfWorkRollsBack (unit_of_work_test.go) and
// TestCreationFailsLeavingOTPReusable
// (creation_transaction_test.go) already make for their own capabilities.
func TestAcceptanceRejectionRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const id = "6a2100000000000000000001"
	seedFleetRequest(t, db, id, "771000001", "771000002")

	uow := persistence.NewUnitOfWork(db)
	boom := errors.New("boom")

	err := uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Requests.RejectNumber(ctx, id, "771000001", ""); err != nil {
			return err
		}
		// Simulates the second write of the closure failing — RejectNumber
		// on "771000002" already succeeded and must not survive.
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	var status string
	if err := db.Pool.QueryRow(ctx,
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = $2",
		id, "771000001").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "EN_COURS" {
		t.Fatalf("number 771000001 survived REJETE despite the rollback (status = %s)", status)
	}
}

// TestAcceptanceRejectRollsBack is the same proof on RequestGateway.Reject
// itself — the two-statement write (etape_historique, then demande) a total
// rejection makes. A failure between the two must leave neither behind.
func TestAcceptanceRejectRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const id = "6a2100000000000000000002"
	seedFleetRequest(t, db, id, "771000003")

	uow := persistence.NewUnitOfWork(db)
	boom := errors.New("boom")

	err := uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Requests.Reject(ctx, id, seed.OperatorOrangeID,
			seed.RejectionReasonMissingDataID, "test", time.Now()); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	var status string
	if err := db.Pool.QueryRow(ctx,
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "EN_COURS" {
		t.Fatalf("the request survived REJETE despite the rollback (status = %s)", status)
	}

	var n int
	if err := db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM etape_historique WHERE demande_id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("the etape_historique row survived the rollback (%d row(s))", n)
	}
}
