package otp_test

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TestVerifyDoesNotConsume pins TC-021: a successful verification leaves the
// code usable, so that creating the request afterwards still works. Only
// failed attempts are counted.
func TestVerifyDoesNotConsume(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
	})

	i := otp.NewVerifyOTP(g, clock, 3)
	if f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"}); f != nil {
		t.Fatalf("expected success, got %v", f)
	}

	stored, _, _ := g.Find(context.Background(), "771000001")
	if stored.Consumed {
		t.Fatal("verification consumed the code; TC-021 says it must not")
	}
}

// TestVerifyCountsOnlyFailures pins the three-attempt limit.
func TestVerifyCountsOnlyFailures(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	for n := 1; n <= 3; n++ {
		if f := i.Execute(context.Background(), otp.VerifyOTPInput{
			MSISDN: "771000001", Code: "000000"}); f.Code != "OTP_INVALID" {
			t.Fatalf("attempt %d: expected OTP_INVALID, got %v", n, f)
		}
	}
	if f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"}); f == nil || f.Code != "OTP_MAX_ATTEMPTS" {
		t.Fatalf("expected OTP_MAX_ATTEMPTS after three failures, got %v", f)
	}
}

// TestVerifyAbsentNumber pins the "no active OTP" branch, reached when the
// gateway reports found=false.
func TestVerifyAbsentNumber(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	i := otp.NewVerifyOTP(g, clock, 3)

	f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "779999999", Code: "123456"})
	if f == nil || f.Code != "OTP_INVALID" {
		t.Fatalf("expected OTP_INVALID (absent), got %v", f)
	}
}

// TestVerifyAlreadyConsumed pins the "consumed" branch and its precedence:
// it fires even though attempts and expiry would otherwise pass.
func TestVerifyAlreadyConsumed(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
		Consumed:  true,
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"})
	if f == nil || f.Code != "OTP_ALREADY_USED" {
		t.Fatalf("expected OTP_ALREADY_USED, got %v", f)
	}
}

// TestVerifyExpired pins the "expired" branch, reached only once consumed
// and max-attempts have both cleared.
func TestVerifyExpired(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(-1 * time.Minute),
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"})
	if f == nil || f.Code != "OTP_EXPIRED" {
		t.Fatalf("expected OTP_EXPIRED, got %v", f)
	}
}

// TestVerifyCheckOrderConsumedBeatsMaxAttempts pins the legacy check order at
// its first two rungs: consumed is tested before max attempts, so a
// consumed OTP that has also exhausted its attempts still reports
// OTP_ALREADY_USED, not OTP_MAX_ATTEMPTS.
func TestVerifyCheckOrderConsumedBeatsMaxAttempts(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
		Attempts:  3,
		Consumed:  true,
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"})
	if f == nil || f.Code != "OTP_ALREADY_USED" {
		t.Fatalf("expected OTP_ALREADY_USED (consumed checked first), got %v", f)
	}
}

// TestVerifyCheckOrderMaxAttemptsBeatsExpiry pins the legacy check order at
// its middle two rungs: max attempts is tested before expiry, so an
// exhausted OTP that has also expired still reports OTP_MAX_ATTEMPTS, not
// OTP_EXPIRED.
func TestVerifyCheckOrderMaxAttemptsBeatsExpiry(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(-1 * time.Minute),
		Attempts:  3,
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"})
	if f == nil || f.Code != "OTP_MAX_ATTEMPTS" {
		t.Fatalf("expected OTP_MAX_ATTEMPTS (checked before expiry), got %v", f)
	}
}

// TestVerifyCheckOrderExpiryBeatsMismatch pins the legacy check order at its
// last two rungs: expiry is tested before the code comparison, so an
// expired OTP presented with the wrong code still reports OTP_EXPIRED, not
// OTP_INVALID — and must not increment the attempt counter.
func TestVerifyCheckOrderExpiryBeatsMismatch(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(-1 * time.Minute),
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "000000"})
	if f == nil || f.Code != "OTP_EXPIRED" {
		t.Fatalf("expected OTP_EXPIRED (checked before comparison), got %v", f)
	}

	stored, _, _ := g.Find(context.Background(), "771000001")
	if stored.Attempts != 0 {
		t.Fatalf("expiry must short-circuit before the attempt is counted, got attempts=%d", stored.Attempts)
	}
}
