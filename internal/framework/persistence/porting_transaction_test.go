//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// seedRequest inserts a bare PARTICULIER/PORTAGE request directly by SQL, at
// the given step — internal/usecase/porting has no Create of its own to
// reuse (it starts from a request creation and acceptance already produced),
// the same level of directness seedFleetRequest (acceptance_transaction_test.go)
// uses.
func seedRequest(t *testing.T, db *persistence.DB, id, msisdn, step string) {
	t.Helper()
	now := time.Now()
	_, err := db.Pool.Exec(context.Background(), `
		INSERT INTO demande
		  (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		   statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		   createur_operateur_id, date_demande, date_debut_etape)
		VALUES ($1, $2, 'PARTICULIER', 'PORTAGE', 'EN_COURS', $3, 'EN_COURS',
		        $4, $5, $5, $6, $6)`,
		id, msisdn, step, seed.OperatorOrangeID, seed.OperatorYASID, now)
	if err != nil {
		t.Fatalf("seed demande : %v", err)
	}
}

// TestPortingConfirmRollsBack proves the guarantee Task 15's ConfirmRequest
// exists to preserve: the confirmation row port.ConfirmationGateway.Confirm
// writes lives inside one port.UnitOfWork.Do, and a failure anywhere else in
// that same closure leaves it undone — even though Confirm itself ran and
// returned successfully before the failure. Without this, "one transaction"
// is a claim rather than a guarantee, the same point
// TestAcceptanceRejectionRollsBack already makes for acceptance's own writes.
func TestPortingConfirmRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const id = "6a2100000000000000000004"
	seedRequest(t, db, id, "771000004", "CONFIRMATION")

	uow := persistence.NewUnitOfWork(db)
	boom := errors.New("boom")

	err := uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Confirmations.Confirm(ctx, id, seed.OperatorOrangeID, "test", time.Now()); err != nil {
			return err
		}
		// Simulates a later write of the same closure failing — the
		// confirmation row above already exists in the transaction's own
		// view and must not survive the rollback.
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	var n int
	if err := db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM confirmation WHERE demande_id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("la confirmation a survécu au rollback (%d ligne(s))", n)
	}
}

// TestPortingProcessStepCommentRollsBack is the same proof on
// RequestGateway.SetComment — the write ProcessStep makes when a caller
// supplies a commentaire, ahead of port.Engine.ScheduleTransition inside the
// same closure. A failure after SetComment must leave the commentaire
// unwritten, not merely unread by a later step.
func TestPortingProcessStepCommentRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const id = "6a2100000000000000000006"
	seedRequest(t, db, id, "771000006", "DESACTIVATION")

	uow := persistence.NewUnitOfWork(db)
	boom := errors.New("boom")

	err := uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Requests.SetComment(ctx, id, "Numéro désactivé"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	var comment *string
	if err := db.Pool.QueryRow(ctx,
		"SELECT commentaire FROM demande WHERE id = $1", id).Scan(&comment); err != nil {
		t.Fatal(err)
	}
	if comment != nil {
		t.Fatalf("le commentaire a survécu au rollback (%q)", *comment)
	}
}

// TestPortingCancelRollsBack is the same proof on RequestGateway.Cancel
// itself — the two-statement write (etape_historique, then demande) a
// cancellation makes, mirroring TestAcceptanceRejectRollsBack for Reject's
// own two statements. A failure between the two must leave neither behind.
func TestPortingCancelRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const id = "6a2100000000000000000005"
	seedRequest(t, db, id, "771000005", "ACCEPTATION")

	uow := persistence.NewUnitOfWork(db)
	boom := errors.New("boom")

	err := uow.Do(ctx, func(repos port.Repositories) error {
		if err := repos.Requests.Cancel(ctx, id, seed.OperatorYASID, entity.StepAcceptance, time.Now()); err != nil {
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
		t.Fatalf("la demande a survécu ANNULE malgré le rollback (statut = %s)", status)
	}

	var n int
	if err := db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM etape_historique WHERE demande_id = $1", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("la ligne etape_historique a survécu au rollback (%d ligne(s))", n)
	}
}
