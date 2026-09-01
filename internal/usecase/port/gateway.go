// Package port declares what the use case layer needs from the outside world.
// Every interface here is implemented in the adapter layer; nothing in this
// package may name pgx, Gin, or any concrete technology.
package port

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// OneTimePassword is the persisted state of an OTP challenge.
type OneTimePassword struct {
	MSISDN    string
	Code      string
	ExpiresAt time.Time
	Attempts  int
	Consumed  bool
}

type OTPGateway interface {
	Upsert(ctx context.Context, otp OneTimePassword) error
	Find(ctx context.Context, msisdn string) (OneTimePassword, bool, error)
	IncrementAttempts(ctx context.Context, msisdn string) error
	Consume(ctx context.Context, msisdn string) error
}

// UserGateway resolves the operator behind a set of credentials, and behind a
// username carried by a token. Both the login use case and the authentication
// middleware go through it.
type UserGateway interface {
	ByCredentials(ctx context.Context, username, password string) (entity.Caller, bool, error)
	ByUsername(ctx context.Context, username string) (entity.Caller, bool, error)
}
