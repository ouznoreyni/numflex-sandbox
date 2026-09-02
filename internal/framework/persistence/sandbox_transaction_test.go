//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// seedPurgeable inserts, directly by SQL, everything one purge touches: an
// already-ported number (owned by ORANGE, origin YAS — the same shape as
// the seed's own 77200 tranche), a demande created by operatorID that
// references it, a live OTP for the same number, and a reverse_request tied
// to the demande — so that a single purge exercises all five tables Task
// 16's own PurgeTestDataInteractor writes.
func seedPurgeable(t *testing.T, db *persistence.DB, requestID, msisdn, operatorID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	portedAt := now.Add(-30 * 24 * time.Hour)

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO numero (msisdn, operateur_actuel_id, operateur_origine_id,
		                     date_dernier_portage, deja_restitue, actif)
		VALUES ($1, $2, $3, $4, false, true)
		ON CONFLICT (msisdn) DO UPDATE
		  SET operateur_actuel_id = EXCLUDED.operateur_actuel_id,
		      operateur_origine_id = EXCLUDED.operateur_origine_id,
		      date_dernier_portage = EXCLUDED.date_dernier_portage`,
		msisdn, seed.OperatorOrangeID, seed.OperatorYASID, portedAt); err != nil {
		t.Fatalf("seed numero : %v", err)
	}

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO demande
		  (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		   statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		   createur_operateur_id, date_demande, date_debut_etape)
		VALUES ($1, $2, 'PARTICULIER', 'PORTAGE', 'EN_COURS', 'ACCEPTATION', 'EN_COURS',
		        $3, $4, $4, $5, $5)`,
		requestID, msisdn, seed.OperatorYASID, operatorID, now); err != nil {
		t.Fatalf("seed demande : %v", err)
	}

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO otp (numero, code, expire_a, tentatives, consomme, cree_le)
		VALUES ($1, '123456', $2, 0, true, $2)`,
		msisdn, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("seed otp : %v", err)
	}

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO reverse_request (id, numero, operateur_id, demande_id, statut, date_demande)
		VALUES ($1, $2, $3, $4, 'EN_ATTENTE', $5)`,
		identifier.New(), msisdn, operatorID, requestID, now); err != nil {
		t.Fatalf("seed reverse_request : %v", err)
	}
}

// TestSandboxPurgeRollsBack proves the guarantee Task 16's PurgeTestData
// exists to preserve: the purge touches five tables (reverse_request, otp,
// demande — plus demande_numero, demande_client and etape_historique by
// cascade — and numero) inside one port.UnitOfWork.Do, and a failure
// anywhere in that closure leaves every one of those writes undone — even
// the ones, like DeleteRequests here, that ran and returned successfully
// before the failure. This is the strongest case for port.UnitOfWork in the
// whole project: without this guarantee, a partial purge could delete a
// request without restoring its number, leaving DELAI_PORTAGE_NON_RESPECTE
// blocking that number for three months with no request left to explain
// why.
func TestSandboxPurgeRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const requestID = "6a2100000000000000000010"
	const msisdn = "779000010"

	seedPurgeable(t, db, requestID, msisdn, seed.OperatorYASID)

	uow := persistence.NewUnitOfWork(db)
	boom := errors.New("boom")

	err := uow.Do(ctx, func(repos port.Repositories) error {
		ids, err := repos.Sandbox.RequestIDsToPurge(ctx, seed.OperatorYASID)
		if err != nil {
			return err
		}
		numbers, err := repos.Sandbox.NumbersToRestore(ctx, ids)
		if err != nil {
			return err
		}
		if _, err := repos.Sandbox.DeleteReverseRequests(ctx, seed.OperatorYASID, ids); err != nil {
			return err
		}
		if _, err := repos.Sandbox.DeleteOTP(ctx, numbers); err != nil {
			return err
		}
		// The request itself is deleted here, inside the transaction's own
		// view — then the closure fails before the registry is restored.
		if _, err := repos.Sandbox.DeleteRequests(ctx, ids); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	var requestCount int
	if err := db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM demande WHERE id = $1", requestID).Scan(&requestCount); err != nil {
		t.Fatal(err)
	}
	if requestCount != 1 {
		t.Fatalf("la demande a survécu supprimée malgré le rollback (%d ligne(s))", requestCount)
	}

	var nOTP int
	if err := db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM otp WHERE numero = $1", msisdn).Scan(&nOTP); err != nil {
		t.Fatal(err)
	}
	if nOTP != 1 {
		t.Fatalf("l'OTP a survécu supprimé malgré le rollback (%d ligne(s))", nOTP)
	}

	var nReverse int
	if err := db.Pool.QueryRow(ctx,
		"SELECT count(*) FROM reverse_request WHERE demande_id = $1", requestID).Scan(&nReverse); err != nil {
		t.Fatal(err)
	}
	if nReverse != 1 {
		t.Fatalf("la demande de reverse a survécu supprimée malgré le rollback (%d ligne(s))", nReverse)
	}

	var currentOperator string
	if err := db.Pool.QueryRow(ctx,
		"SELECT operateur_actuel_id FROM numero WHERE msisdn = $1", msisdn).Scan(&currentOperator); err != nil {
		t.Fatal(err)
	}
	if currentOperator != seed.OperatorOrangeID {
		t.Fatalf("le registre a été restauré malgré le rollback (operateur_actuel_id = %s)", currentOperator)
	}
}
