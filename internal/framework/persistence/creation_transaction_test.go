//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/creation"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TestCreationEchoueLaissantLOTPReutilisable is Task 12's own version of
// TestUnitOfWorkRollsBack, run through the real production wiring instead of
// a synthetic "boom" error: CreateIndividualRequestInteractor, the real
// Postgres-backed port.UnitOfWork, and the real NumberGateway/RequestGateway/
// OTPGateway. It proves the one guarantee this task exists to preserve
// (commit 643415f): OTP.Consume is the last call inside the transaction, so
// when an earlier write fails, the whole transaction — request row included
// — rolls back and the OTP stays consumable.
//
// The failure is forced by handing a syntactically valid but non-existent
// RecipientOperatorID: it passes every business check (eligibility never
// inspects the recipient row), reaches repos.Requests.Create inside Do, and
// only then trips the operateur_destinataire_id foreign key — exactly the
// kind of failure a real bug (a stale caller identity, a deleted operator)
// could produce in production.
func TestCreationEchoueLaissantLOTPReutilisable(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	const msisdn = "771000001" // seed : ORANGE, jamais porté
	const codeInvalide = "operateur-inexistant"

	otpGw := postgres.NewOTPGateway(db.Pool)
	if err := otpGw.Upsert(ctx, port.OneTimePassword{
		MSISDN: msisdn, Code: "123456", ExpiresAt: time.Now().Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	numbers := postgres.NewNumberGateway(db.Pool)
	requests := postgres.NewRequestGateway(db.Pool)
	uow := persistence.NewUnitOfWork(db)
	clk := clock.New(0)
	ids := identifier.NewGenerator()
	verify := otp.NewVerifyOTP(otpGw, clk, 3)

	interactor := creation.NewCreateIndividualRequest(verify, numbers, uow, requests, ids, clk)

	// Le contexte porte un Caller dont l'OperatorID est le même identifiant
	// invalide que RecipientOperatorID : la vérification d'autorisation
	// passe (les deux sont égaux), et l'échec ne survient qu'à l'écriture.
	callerCtx := port.WithCaller(ctx, entity.Caller{OperatorID: codeInvalide})

	_, fault := interactor.Execute(callerCtx, creation.CreateIndividualRequestInput{
		MSISDN: msisdn, OTPCode: "123456",
		SourceOperatorID: seed.OperateurOrange, RecipientOperatorID: codeInvalide,
		Processus: "PREPAID",
		Client: creation.ClientInput{
			LastName: "Diallo", FirstName: "Mamadou", BirthDate: "1975-03-20",
			BirthPlace: "Dakar", IDType: "CNI", IDNumber: "123",
		},
	})
	if fault == nil {
		t.Fatal("la création aurait dû échouer (operateur_destinataire_id invalide)")
	}

	// Le rollback a tout défait : aucune demande n'existe.
	var n int
	if err := db.Pool.QueryRow(ctx, "SELECT count(*) FROM demande").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("la transaction a laissé %d demande(s) derrière elle", n)
	}

	// L'OTP n'a pas été consommé : Consume est le dernier appel de la
	// transaction, jamais atteint puisque Create a échoué avant lui.
	stored, found, err := otpGw.Find(ctx, msisdn)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("l'OTP a disparu")
	}
	if stored.Consumed {
		t.Fatal("l'OTP a été consommé malgré l'échec de la transaction")
	}

	// Preuve positive, pas seulement l'absence du drapeau : le même code
	// redevient effectivement utilisable pour une nouvelle tentative.
	if f := verify.Execute(ctx, otp.VerifyOTPInput{MSISDN: msisdn, Code: "123456"}); f != nil {
		t.Fatalf("l'OTP devrait rester valide après le rollback : %v", f)
	}
}
