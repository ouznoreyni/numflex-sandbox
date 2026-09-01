//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// TestOTPGatewayUpsertResetsAttempts pins the ON CONFLICT clause: a second
// send on a number that already failed attempts AND was consumed must reset
// both tentatives and consomme, otherwise a user could be locked out after
// three failures forever, or could never be re-challenged after one
// successful flow. Both Consume and IncrementAttempts are exercised first so
// the row genuinely holds a non-default value on each field before the
// reset — otherwise the final assertion on consomme would be trivially true.
func TestOTPGatewayUpsertResetsAttempts(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewOTPGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))
	must(t, g.IncrementAttempts(ctx, "771000001"))
	must(t, g.Consume(ctx, "771000001"))

	// Precondition: both fields genuinely hold a non-default value before the
	// reset, or the assertions below would prove nothing.
	before, found, err := g.Find(ctx, "771000001")
	must(t, err)
	if !found {
		t.Fatal("otp not found")
	}
	if before.Attempts == 0 || !before.Consumed {
		t.Fatalf("precondition failed: attempts=%d consumed=%v, want attempts>0 and consumed=true",
			before.Attempts, before.Consumed)
	}

	// A second send must reset attempts and the consumed flag: the ON CONFLICT
	// clause is what makes a number re-challengeable.
	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))

	got, found, err := g.Find(ctx, "771000001")
	must(t, err)
	if !found {
		t.Fatal("otp not found")
	}
	if got.Attempts != 0 || got.Consumed {
		t.Fatalf("upsert did not reset: attempts=%d consumed=%v", got.Attempts, got.Consumed)
	}
}

// TestOTPGatewayFindUnknownNumber asserts Find reports absence through its
// bool return, not through an error — callers (OTPAbsent) rely on this.
func TestOTPGatewayFindUnknownNumber(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewOTPGateway(db.Pool)
	ctx := context.Background()

	got, found, err := g.Find(ctx, "779999999")
	must(t, err)
	if found {
		t.Fatalf("expected not found, got %+v", got)
	}
}

// TestOTPGatewayFindReturnsStoredFields asserts every column Find selects
// round-trips through the gateway unchanged.
func TestOTPGatewayFindReturnsStoredFields(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewOTPGateway(db.Pool)
	ctx := context.Background()

	expiry := time.Now().Add(5 * time.Minute).Truncate(time.Microsecond)
	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000002", Code: "654321", ExpiresAt: expiry,
	}))

	got, found, err := g.Find(ctx, "771000002")
	must(t, err)
	if !found {
		t.Fatal("otp not found")
	}
	if got.Code != "654321" {
		t.Fatalf("code = %q, want 654321", got.Code)
	}
	if !got.ExpiresAt.Equal(expiry) {
		t.Fatalf("expiresAt = %v, want %v", got.ExpiresAt, expiry)
	}
	if got.Attempts != 0 || got.Consumed {
		t.Fatalf("fresh otp should start unattempted and unconsumed, got attempts=%d consumed=%v",
			got.Attempts, got.Consumed)
	}
}

// TestOTPGatewayIncrementAttempts pins the three-attempt limit's counter:
// each call must add exactly one, and be visible to a subsequent Find.
func TestOTPGatewayIncrementAttempts(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewOTPGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000003", Code: "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))

	for i := 1; i <= 2; i++ {
		must(t, g.IncrementAttempts(ctx, "771000003"))
		got, found, err := g.Find(ctx, "771000003")
		must(t, err)
		if !found {
			t.Fatal("otp not found")
		}
		if got.Attempts != i {
			t.Fatalf("after %d increment(s): attempts = %d, want %d", i, got.Attempts, i)
		}
	}
}

// TestOTPGatewayConsume pins TC-021's counterpart: Consume is the only path
// that sets consomme — verify pre-checks without calling it.
func TestOTPGatewayConsume(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewOTPGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000004", Code: "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))
	must(t, g.Consume(ctx, "771000004"))

	got, found, err := g.Find(ctx, "771000004")
	must(t, err)
	if !found {
		t.Fatal("otp not found")
	}
	if !got.Consumed {
		t.Fatal("expected consomme = true after Consume")
	}
}
