// Package port declares what the use case layer needs from the outside world.
// Every interface here is implemented in the adapter layer; nothing in this
// package may name pgx, Gin, or any concrete technology.
package port

import (
	"context"
	"errors"
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
	// RejectionReasonExists answers whether id names a row in motif_rejet —
	// acceptance's guard on a caller-supplied motifRejetId (Task 14),
	// top-level on both routes and per-number on a fleet's partial
	// rejection. A dedicated existence check rather than a call to
	// RejectionReasons filtered in memory: the same division RequestGateway
	// already draws between Get (a full read-back) and its narrower
	// existence-shaped reads like RoutingPrefix.
	RejectionReasonExists(ctx context.Context, id string) (bool, error)
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

	// ByID reads the authorization-relevant shape of a request —
	// entity.PortingRequest, the same six columns the legacy handlers'
	// chargerDemande loaded — regardless of which queue it sits in today.
	// Task 14 (acceptance) is its first caller: entity.CanAccept needs the
	// whole request, not the queue-filtered view QueryGateway.ByID answers
	// (that one folds "wrong operator" into RequestNotFound; acceptance
	// must still tell the two apart, per TC-034). Read outside any
	// transaction, exactly as chargerDemande was: before acceptance opens
	// the transaction that changes state, never inside it.
	ByID(ctx context.Context, id string) (entity.PortingRequest, bool, error)

	// SetComment writes a request's free-text commentaire without moving it
	// out of its current step — the write an acceptance (individual, or a
	// fleet with numbers still active) makes before scheduling its
	// transition.
	SetComment(ctx context.Context, id, comment string) error

	// NumberBelongs answers whether msisdn is one of request id's
	// demande_numero rows — the fleet-rejection guard: a numerosRejetes
	// entry naming a number outside the request is refused before anything
	// is written.
	NumberBelongs(ctx context.Context, requestID, msisdn string) (bool, error)

	// RejectNumber marks one fleet member REJETE, recording rejectionReasonID
	// when one is given (NULL otherwise).
	RejectNumber(ctx context.Context, requestID, msisdn, rejectionReasonID string) error

	// HasActiveNumber answers whether request id still has at least one
	// demande_numero row that is not REJETE — the fleet-rejection guard
	// deciding whether a partially-rejected fleet still has something to
	// port, or has been rejected numéro by numéro until nothing is left.
	HasActiveNumber(ctx context.Context, requestID string) (bool, error)

	// Reject closes a request definitively: REJETE, its current step marked
	// TERMINE, the rejection reason recorded, and one etape_historique row
	// of origin ACTION — no transition of the engine ever writes this one,
	// since R-10 only governs acceptance's own convergence. Shared by an
	// individual rejection and a fleet rejected in full, either outright or
	// numéro by numéro until nothing is left ([HYP], see
	// internal/usecase/acceptance).
	Reject(ctx context.Context, requestID, operatorID, rejectionReasonID, comment string, now time.Time) error

	// Cancel withdraws a request before it has moved (§7.11): the same
	// terminal etape_historique row Reject writes, then the demande row
	// itself marked ANNULE with nothing left pending — moved verbatim from
	// the deleted internal/api/annulation.go's postAnnuler, which opened its
	// own *pgx.Tx for exactly these two statements. Task 15
	// (internal/usecase/porting) is its first caller.
	Cancel(ctx context.Context, requestID, operatorID string, now time.Time) error
}

// ErrAlreadyConfirmed is ConfirmationGateway.Confirm's answer to a replay:
// operatorID has already confirmed requestID. It is a sentinel rather than a
// raw SQL error code because the anti-replay guarantee comes from the
// confirmation table's own primary key (demande_id, operateur_id), not from
// a pre-check — a pre-check would race two concurrent calls from the same
// operator — and no *pgconn.PgError may cross into internal/usecase: only
// the Postgres gateway may know that code 23505 means this.
var ErrAlreadyConfirmed = errors.New("l'opérateur a déjà confirmé cette demande")

// ConfirmationGateway records one operator's confirmation at the
// CONFIRMATION step, and counts how many a request has received so far —
// the two operations POST /demandes/a-confirmer needs beyond
// entity.ExpectedConfirmers, which decides who must confirm but not how
// many already have.
type ConfirmationGateway interface {
	// Confirm inserts one confirmation row, or returns ErrAlreadyConfirmed
	// if operatorID already confirmed requestID.
	Confirm(ctx context.Context, requestID, operatorID, comment string, now time.Time) error
	// Count answers how many operators have confirmed requestID so far.
	Count(ctx context.Context, requestID string) (int, error)
}

// Queue names the three read-only queues that carry a single-id detail
// route (/a-accepter/:id, /a-traiter/:id, /a-confirmer/:id) alongside their
// list route. Own, AlreadyConfirmed, Incoming and Outgoing have no detail
// route in the guide, so they never appear as a QueryGateway.ByID argument.
type Queue string

const (
	QueueToAccept  Queue = "a-accepter"
	QueueToProcess Queue = "a-traiter"
	QueueToConfirm Queue = "a-confirmer"
)

// QueryGateway resolves the seven read-only queues behind
// GET /demandes/{mes-demandes,a-accepter,a-traiter,a-confirmer,
// deja-confirmees,in,out} — internal/api/demandes_lecture.go before this
// task. Every list method returns ids ordered by date_demande, exactly as
// the legacy handlers' shared idsDemandes did: turning an id into a
// renderable view stays RequestGateway.Get's job, already built for request
// creation and reused here rather than duplicated (see
// internal/usecase/query).
type QueryGateway interface {
	// Own lists the requests where operatorID is source or recipient
	// (GET /mes-demandes).
	Own(ctx context.Context, operatorID string) ([]string, error)
	// ToAccept lists the EN_COURS/ACCEPTATION requests operatorID is source
	// of (GET /a-accepter).
	ToAccept(ctx context.Context, operatorID string) ([]string, error)
	// ToProcess lists the requests entity.StepOwner(step, type) makes
	// operatorID responsible for right now (GET /a-traiter).
	ToProcess(ctx context.Context, operatorID string) ([]string, error)
	// ToConfirm lists the EN_COURS/CONFIRMATION requests operatorID is an
	// expected confirmer of and has not yet confirmed
	// (entity.ExpectedConfirmers) (GET /a-confirmer).
	ToConfirm(ctx context.Context, operatorID string) ([]string, error)
	// AlreadyConfirmed lists the requests operatorID has confirmed
	// (GET /deja-confirmees). excludeSource reproduces ANO-019: in real
	// fidelity, the platform omits confirmations issued by the request's
	// own source operator — measured, absent from the contract guide. The
	// bool crosses from internal/framework/config.Fidelity at the wiring
	// layer (internal/api), which this package may not import.
	AlreadyConfirmed(ctx context.Context, operatorID string, excludeSource bool) ([]string, error)
	// Incoming lists the completed PORTAGE requests where operatorID is
	// recipient (GET /in).
	Incoming(ctx context.Context, operatorID string) ([]string, error)
	// Outgoing lists the completed PORTAGE requests where operatorID is
	// source (GET /out).
	Outgoing(ctx context.Context, operatorID string) ([]string, error)

	// ByID answers whether id belongs to queue for operatorID — the same
	// predicate the matching list method applies, reapplied to a single
	// row so a detail route needn't load every id first. found is false
	// both when id doesn't exist at all and when it exists but isn't in
	// this queue for this caller: the legacy handlers never told the two
	// apart (both end in entity.RequestNotFound), and this preserves that.
	ByID(ctx context.Context, queue Queue, id, operatorID string) (entity.PortingRequest, bool, error)
}
