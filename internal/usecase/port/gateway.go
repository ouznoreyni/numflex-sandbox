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

// NumberGateway resolves a number's current standing in the registry — the
// same read the three creation endpoints (and later acceptance, restitution
// and reverse) need before deciding whether a number is portable. It is
// always read outside any transaction: nothing here is written by request
// creation.
type NumberGateway interface {
	// State returns found=false when msisdn is absent from the registry —
	// the caller decides what fault that deserves (a source number must be
	// registered; a fleet member that isn't cannot be requested either).
	State(ctx context.Context, msisdn string) (entity.NumberState, bool, error)
}

// CreateRequestInput carries the fields a new porting/restitution request
// persists into the demande table. Processus and RoutingInfo are nil for a
// restitution, which has neither dimension before its own COMPLETION.
type CreateRequestInput struct {
	ID                  string
	MSISDN              string
	SubscriberType      string
	RequestType         string
	SourceOperatorID    string
	RecipientOperatorID string
	CreatorOperatorID   string
	Processus           *string
	RoutingInfo         *string
	RequestDate         time.Time
}

// RequestNumberInput carries one retained number of a request — the sole
// number of a particulier/restitution request, or one member of a fleet.
// RoutingInfo is nil for a restitution.
type RequestNumberInput struct {
	RequestID   string
	MSISDN      string
	RoutingInfo *string
}

// ExcludedNumberInput carries one fleet number rejected at creation time —
// BR-006, invariant 11: the fleet still succeeds, minus this number.
type ExcludedNumberInput struct {
	RequestID string
	MSISDN    string
	Reason    string
	ErrorCode string
}

// ClientInput carries the identity attached to a request. CompanyName and
// RCNumber are nil outside an ENTREPRISE request; a restitution has no
// client row at all.
type ClientInput struct {
	RequestID   string
	LastName    string
	FirstName   string
	BirthDate   string // ISO yyyy-mm-dd, bound straight from the request JSON
	BirthPlace  string
	IDType      string
	IDNumber    string
	CompanyName *string
	RCNumber    *string
}

// ClientView is the identity attached to a request, as read back right after
// its creation.
type ClientView struct {
	LastName, FirstName, BirthPlace, IDType, IDNumber string
	BirthDate                                         *time.Time
}

// RequestView is a request as read back right after its creation, in the
// shape the particulier and restitution endpoints render (guide §7.3). The
// fleet endpoint needs no read-back: its response is built entirely from
// what the interactor already knows.
type RequestView struct {
	ID                                         string
	MSISDN                                     string
	SubscriberType, RequestType, Status        string
	CurrentStep, CurrentStepStatus             string
	SourceOperatorID, SourceOperatorName       string
	RecipientOperatorID, RecipientOperatorName string
	RequestDate                                time.Time
	Processus, RoutingInfo                     *string
	CompletionDate                             *time.Time
	Client                                     *ClientView
}

// RequestGateway persists a new porting/restitution request and its
// associated rows, and reads one back right after creation. RoutingPrefix,
// Create, AddNumber, AddExcludedNumber and AddClient are always called
// inside a port.UnitOfWork transaction (via Repositories.Requests), bound to
// the same transaction as the OTP consumption they must survive or fail
// with; Get is called afterwards, against the plain pool, exactly as the
// legacy handlers' post-commit demandeDTO read was.
type RequestGateway interface {
	// RoutingPrefix resolves an operator's prefixe_routage, or an error if
	// operatorID does not exist — the caller turns that into
	// entity.ValidationFailed("Opérateur source inconnu"), matching the
	// legacy handlers exactly.
	RoutingPrefix(ctx context.Context, operatorID string) (string, error)
	Create(ctx context.Context, in CreateRequestInput) error
	AddNumber(ctx context.Context, in RequestNumberInput) error
	AddExcludedNumber(ctx context.Context, in ExcludedNumberInput) error
	AddClient(ctx context.Context, in ClientInput) error
	Get(ctx context.Context, id string) (RequestView, bool, error)
}
