package creation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/creation"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// errFake stands in for a gateway-level failure (e.g. a real FK violation)
// without depending on any concrete error type.
var errFake = errors.New("échec simulé de la couche gateway")

const (
	orangeID = "operateur-orange"
	yasID    = "operateur-yas"
)

// fixture bundles the doubles a creation interactor test wires — one set per
// test, since each gets its own maps and cannot leak state across tests.
type fixture struct {
	otp      *inmemory.OTPGateway
	numbers  *inmemory.NumberGateway
	requests *inmemory.RequestGateway
	uow      *inmemory.UnitOfWork
	ids      *inmemory.IDGenerator
	clock    inmemory.FixedClock
}

func newFixture() *fixture {
	otpGw := inmemory.NewOTPGateway()
	requests := inmemory.NewRequestGateway()
	return &fixture{
		otp:      otpGw,
		numbers:  inmemory.NewNumberGateway(),
		requests: requests,
		uow:      inmemory.NewUnitOfWork(port.Repositories{OTP: otpGw, Requests: requests}),
		ids:      inmemory.NewIDGenerator(),
		clock:    inmemory.FixedClock{At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)},
	}
}

func ctxCaller(operatorID string) context.Context {
	return port.WithCaller(context.Background(), entity.Caller{OperatorID: operatorID})
}

func individualInteractor(f *fixture) *creation.CreateIndividualRequestInteractor {
	verify := otp.NewVerifyOTP(f.otp, f.clock, 3)
	return creation.NewCreateIndividualRequest(verify, f.numbers, f.uow, f.requests, f.ids, f.clock)
}

func seedOTP(t *testing.T, f *fixture, msisdn, code string) {
	t.Helper()
	require.NoError(t, f.otp.Upsert(context.Background(), port.OneTimePassword{
		MSISDN: msisdn, Code: code, ExpiresAt: f.clock.Now().Add(5 * time.Minute),
	}))
}

func validIndividualInput(msisdn string) creation.CreateIndividualRequestInput {
	return creation.CreateIndividualRequestInput{
		MSISDN: msisdn, OTPCode: "123456",
		SourceOperatorID: orangeID, RecipientOperatorID: yasID,
		Process: "PREPAID",
		Client: creation.ClientInput{
			LastName: "Diallo", FirstName: "Mamadou", BirthDate: "1975-03-20",
			BirthPlace: "Dakar", IDType: "CNI", IDNumber: "123",
		},
	}
}

func TestCreateIndividualRequestNominal(t *testing.T) {
	f := newFixture()
	f.requests.SeedPrefix(orangeID, "191")
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	seedOTP(t, f, "771000001", "123456")

	view, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.Nil(t, fault)
	require.Equal(t, "771000001", view.MSISDN)
	require.Equal(t, "PARTICULIER", view.SubscriberType)
	require.Equal(t, "PORTAGE", view.RequestType)
	require.NotNil(t, view.RoutingInfo)
	require.Equal(t, "191", *view.RoutingInfo)
	require.NotNil(t, view.Process)
	require.Equal(t, "PREPAID", *view.Process)
	require.NotNil(t, view.Client)
	require.Equal(t, "Diallo", view.Client.LastName)

	// The OTP is consumed — last proof that the transaction really went all
	// the way through.
	stored, found, err := f.otp.Find(context.Background(), "771000001")
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, stored.Consumed)
}

func TestCreateIndividualRequestRefusedIfNotRecipient(t *testing.T) {
	f := newFixture()
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	seedOTP(t, f, "771000001", "123456")

	// The caller is ORANGE, but operateurDestinataireId is YAS.
	_, fault := individualInteractor(f).Execute(ctxCaller(orangeID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)

	// No write: the refusal happens before any OTP check.
	require.Equal(t, 0, f.requests.RequestCount())
}

func TestCreateIndividualRequestInvalidOTPConsumesNothing(t *testing.T) {
	f := newFixture()
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	seedOTP(t, f, "771000001", "999999") // different stored code

	_, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "OTP_INVALID", fault.Code)
	require.Equal(t, 0, f.requests.RequestCount())
}

func TestCreateIndividualRequestUnknownNumber(t *testing.T) {
	f := newFixture()
	seedOTP(t, f, "771000001", "123456")
	// No numero.Seed: the registry does not know this number.

	_, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "OPERATEUR_SOURCE_INCORRECT", fault.Code)
}

func TestCreateIndividualRequestPortingDelayNotRespected(t *testing.T) {
	f := newFixture()
	recent := time.Now().Add(-24 * time.Hour) // ported yesterday: well under 3 months
	f.numbers.Seed(entity.NumberState{
		MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID,
		LastPortingDate: &recent,
	})
	seedOTP(t, f, "771000001", "123456")

	_, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", fault.Code)
	require.Equal(t, 0, f.requests.RequestCount())
}

// TestCreateIndividualRequestWriteFailureDoesNotConsumeOTP is the proof, at
// the interactor level, that OTP.Consume is never reached when one of the
// previous write calls fails inside the same transaction — the guarantee of
// commit 643415f. The proof that a REAL UnitOfWork really undoes its writes
// on that same path lives in internal/framework/persistence (Postgres,
// //go:build integration): this test proves the ORDER of the calls, not the
// rollback itself, which the in-memory UnitOfWork cannot simulate.
func TestCreateIndividualRequestWriteFailureDoesNotConsumeOTP(t *testing.T) {
	f := newFixture()
	f.requests.SeedPrefix(orangeID, "191")
	f.requests.FailCreate = errFake
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	seedOTP(t, f, "771000001", "123456")

	_, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)

	stored, found, err := f.otp.Find(context.Background(), "771000001")
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, stored.Consumed, "the OTP must not be consumed when creation fails")
}
