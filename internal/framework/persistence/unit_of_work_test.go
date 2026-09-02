//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TestUnitOfWorkRollsBack proves the transaction boundary is real: a failure
// inside Do leaves nothing behind. Without this, "one transaction" is a claim
// rather than a guarantee.
//
// R5: the read-back goes through db.Pool.QueryRow directly rather than
// postgres.NewOTPGateway — that gateway constructor does not exist until
// Task 7, and this task must not anticipate it.
func TestUnitOfWorkRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	uow := persistence.NewUnitOfWork(db)

	boom := errors.New("boom")
	err := uow.Do(context.Background(), func(r port.Repositories) error {
		if err := r.OTP.Upsert(context.Background(), port.OneTimePassword{
			MSISDN: "771000001", Code: "123456",
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	// Read back outside any transaction, directly against the pool.
	var n int
	if err := db.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM otp WHERE numero = $1", "771000001").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("the OTP survived a rolled-back transaction")
	}
}

// TestUnitOfWorkCommits proves the other half of the boundary: a successful
// fn leaves its writes visible once Do returns.
func TestUnitOfWorkCommits(t *testing.T) {
	db := testsupport.NewTestDB(t)
	uow := persistence.NewUnitOfWork(db)

	err := uow.Do(context.Background(), func(r port.Repositories) error {
		return r.OTP.Upsert(context.Background(), port.OneTimePassword{
			MSISDN: "771000002", Code: "654321",
		})
	})
	if err != nil {
		t.Fatalf("expected commit, got %v", err)
	}

	var n int
	if err := db.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM otp WHERE numero = $1", "771000002").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("the OTP did not survive a committed transaction")
	}
}
