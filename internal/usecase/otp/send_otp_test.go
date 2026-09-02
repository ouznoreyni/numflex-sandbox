package otp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

var errBoom = errors.New("boom")

// failingOTPGateway is a minimal port.OTPGateway double whose Upsert always
// fails, to check the interactor does not swallow a gateway error.
type failingOTPGateway struct {
	err error
}

func (g failingOTPGateway) Upsert(context.Context, port.OneTimePassword) error { return g.err }

func (g failingOTPGateway) Find(context.Context, string) (port.OneTimePassword, bool, error) {
	return port.OneTimePassword{}, false, nil
}

func (g failingOTPGateway) IncrementAttempts(context.Context, string) error { return nil }

func (g failingOTPGateway) Consume(context.Context, string) error { return nil }

// TestSendOTPPersistsChallenge pins the shape of what Send writes: the
// configured static code, an expiry computed from the injected clock plus
// the configured TTL, and a fresh (unattempted, unconsumed) record.
func TestSendOTPPersistsChallenge(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	i := otp.NewSendOTP(g, clock, "123456", 5*time.Minute)

	if err := i.Execute(context.Background(), otp.SendOTPInput{MSISDN: "771000001"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	stored, found, err := g.Find(context.Background(), "771000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected the OTP to be stored")
	}
	if stored.Code != "123456" {
		t.Fatalf("code = %q, want 123456", stored.Code)
	}
	wantExpiry := clock.Now().Add(5 * time.Minute)
	if !stored.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expiresAt = %v, want %v", stored.ExpiresAt, wantExpiry)
	}
	if stored.Attempts != 0 || stored.Consumed {
		t.Fatalf("expected a fresh challenge, got attempts=%d consumed=%v", stored.Attempts, stored.Consumed)
	}
}

// TestSendOTPResendResetsAttemptsAndConsumed exercises the same guarantee as
// the gateway's ON CONFLICT clause, but at the interactor level: a resend on
// a number that already failed attempts (or was consumed) must reset both,
// re-making the number challengeable — otherwise a user locked out after
// three failures would stay locked out forever.
func TestSendOTPResendResetsAttemptsAndConsumed(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "999999",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
		Attempts:  2, Consumed: true,
	})

	i := otp.NewSendOTP(g, clock, "123456", 5*time.Minute)
	if err := i.Execute(context.Background(), otp.SendOTPInput{MSISDN: "771000001"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	stored, found, err := g.Find(context.Background(), "771000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected the OTP to be stored")
	}
	if stored.Attempts != 0 || stored.Consumed {
		t.Fatalf("resend must reset attempts and consumed, got attempts=%d consumed=%v",
			stored.Attempts, stored.Consumed)
	}
	if stored.Code != "123456" {
		t.Fatalf("resend must replace the code, got %q", stored.Code)
	}
}

// TestSendOTPPropagatesGatewayError asserts a gateway failure surfaces
// unchanged: the interactor adds no error handling of its own.
func TestSendOTPPropagatesGatewayError(t *testing.T) {
	g := failingOTPGateway{err: errBoom}
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	i := otp.NewSendOTP(g, clock, "123456", 5*time.Minute)

	if err := i.Execute(context.Background(), otp.SendOTPInput{MSISDN: "771000001"}); err != errBoom {
		t.Fatalf("expected errBoom to propagate, got %v", err)
	}
}
