// Package otp holds the OTP use cases: sending a challenge and verifying it.
// It depends on nothing but stdlib, internal/entity and internal/usecase/port
// — no pgx, no Gin, no concrete technology.
package otp

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// SendOTPInput carries the number to challenge.
type SendOTPInput struct {
	MSISDN string
}

// SendOTPBoundary is the interface a controller drives; it exists so the
// controller can depend on the use case's contract, not its struct.
type SendOTPBoundary interface {
	Execute(context.Context, SendOTPInput) error
}

// SendOTPInteractor implements SendOTPBoundary. The sandbox does not send an
// SMS: code is the fixed value configured for the environment, logged rather
// than delivered — the response acknowledges submission, not delivery
// (ANO-021).
type SendOTPInteractor struct {
	gateway port.OTPGateway
	clock   port.Clock
	code    string
	ttl     time.Duration
}

// NewSendOTP wires an interactor against the given gateway and clock, with
// the static code and time-to-live pulled from configuration.
func NewSendOTP(g port.OTPGateway, c port.Clock, code string, ttl time.Duration) *SendOTPInteractor {
	return &SendOTPInteractor{gateway: g, clock: c, code: code, ttl: ttl}
}

// Execute (re)issues a challenge for in.MSISDN. The gateway's Upsert carries
// the ON CONFLICT reset of tentatives and consomme, so a resend always makes
// the number re-challengeable regardless of its prior attempt history.
func (i *SendOTPInteractor) Execute(ctx context.Context, in SendOTPInput) error {
	now := i.clock.Now()
	return i.gateway.Upsert(ctx, port.OneTimePassword{
		MSISDN:    in.MSISDN,
		Code:      i.code,
		ExpiresAt: now.Add(i.ttl),
	})
}
