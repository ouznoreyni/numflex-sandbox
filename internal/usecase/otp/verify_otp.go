package otp

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// VerifyOTPInput carries the number and the code presented for it.
type VerifyOTPInput struct {
	MSISDN string
	Code   string
}

// VerifyOTPInteractor pre-verifies without consuming (TC-021): the code
// stays usable so the request creation that follows can still succeed. Only
// failed attempts are counted.
type VerifyOTPInteractor struct {
	gateway     port.OTPGateway
	clock       port.Clock
	maxAttempts int
}

// NewVerifyOTP wires an interactor against the given gateway and clock, with
// the maximum number of allowed attempts pulled from configuration.
func NewVerifyOTP(g port.OTPGateway, c port.Clock, maxAttempts int) *VerifyOTPInteractor {
	return &VerifyOTPInteractor{gateway: g, clock: c, maxAttempts: maxAttempts}
}

// Execute is the interactor counterpart of the former Deps.verifierOTP. The
// order of checks is load-bearing and stays exactly as it was: consumed,
// then max attempts, then expiry, then code mismatch.
func (i *VerifyOTPInteractor) Execute(ctx context.Context, in VerifyOTPInput) *entity.Fault {
	stored, found, err := i.gateway.Find(ctx, in.MSISDN)
	if err != nil {
		return entity.InternalError("lecture de l'OTP")
	}
	if !found {
		return entity.OTPAbsent()
	}

	if stored.Consumed {
		return entity.OTPAlreadyUsed()
	}
	if stored.Attempts >= i.maxAttempts {
		return entity.OTPMaxAttempts()
	}
	if i.clock.Now().After(stored.ExpiresAt) {
		return entity.OTPExpired()
	}
	if in.Code != stored.Code {
		// L'échec de cet incrément ne peut pas être avalé : sans lui, la limite
		// de trois tentatives cesse silencieusement de s'appliquer.
		if err := i.gateway.IncrementAttempts(ctx, in.MSISDN); err != nil {
			return entity.InternalError("incrément des tentatives OTP")
		}
		return entity.OTPInvalid()
	}
	return nil
}
