// Package inmemory holds map-backed doubles of the usecase/port interfaces,
// for use case unit tests that must not touch a database.
package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// OTPGateway is a map-backed port.OTPGateway, keyed by MSISDN.
type OTPGateway struct {
	mu   sync.Mutex
	otps map[string]port.OneTimePassword
}

// NewOTPGateway returns an empty double, ready to use.
func NewOTPGateway() *OTPGateway {
	return &OTPGateway{otps: make(map[string]port.OneTimePassword)}
}

// Seed pre-loads an OTP directly, bypassing any use-case logic — interactor
// tests use it to set up a starting state (attempts already spent, an
// already-consumed code, an expired timestamp) that Execute itself would
// never produce.
func (g *OTPGateway) Seed(otp port.OneTimePassword) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.otps == nil {
		g.otps = make(map[string]port.OneTimePassword)
	}
	g.otps[otp.MSISDN] = otp
}

func (g *OTPGateway) Upsert(_ context.Context, otp port.OneTimePassword) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.otps == nil {
		g.otps = make(map[string]port.OneTimePassword)
	}
	g.otps[otp.MSISDN] = otp
	return nil
}

func (g *OTPGateway) Find(_ context.Context, msisdn string) (port.OneTimePassword, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	otp, ok := g.otps[msisdn]
	return otp, ok, nil
}

func (g *OTPGateway) IncrementAttempts(_ context.Context, msisdn string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	otp, ok := g.otps[msisdn]
	if !ok {
		return nil
	}
	otp.Attempts++
	g.otps[msisdn] = otp
	return nil
}

func (g *OTPGateway) Consume(_ context.Context, msisdn string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	otp, ok := g.otps[msisdn]
	if !ok {
		return nil
	}
	otp.Consumed = true
	g.otps[msisdn] = otp
	return nil
}

// FixedClock is a port.Clock that never moves, for deterministic tests.
type FixedClock struct {
	At   time.Time
	Skew time.Duration
}

func (c FixedClock) Now() time.Time { return c.At }

func (c FixedClock) Rendered(t time.Time) time.Time { return t.Add(c.Skew) }
