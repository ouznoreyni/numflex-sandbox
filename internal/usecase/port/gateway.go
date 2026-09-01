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

// ReferenceGateway resolves the five read-only reference lists behind
// /operateurs, /motifs-rejet, /types-demande, /processus and /types-incident.
// Each method returns a full, already-ordered list: there is no filtering or
// paging in the guide for any of these five endpoints.
type ReferenceGateway interface {
	Operators(ctx context.Context) ([]entity.Operator, error)
	RejectionReasons(ctx context.Context) ([]entity.RejectionReason, error)
	RequestTypes(ctx context.Context) ([]entity.RequestTypeRef, error)
	Processes(ctx context.Context) ([]entity.Process, error)
	IncidentTypes(ctx context.Context) ([]entity.IncidentType, error)
}
