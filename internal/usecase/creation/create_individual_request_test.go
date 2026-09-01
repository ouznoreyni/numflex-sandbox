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
		Processus: "PREPAID",
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
	require.NotNil(t, view.Processus)
	require.Equal(t, "PREPAID", *view.Processus)
	require.NotNil(t, view.Client)
	require.Equal(t, "Diallo", view.Client.LastName)

	// L'OTP est consommé — dernière preuve que la transaction est bien allée
	// jusqu'au bout.
	stored, found, err := f.otp.Find(context.Background(), "771000001")
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, stored.Consumed)
}

func TestCreateIndividualRequestRefuseSiPasDestinataire(t *testing.T) {
	f := newFixture()
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	seedOTP(t, f, "771000001", "123456")

	// L'appelant est ORANGE, mais operateurDestinataireId vaut YAS.
	_, fault := individualInteractor(f).Execute(ctxCaller(orangeID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)

	// Aucune écriture : le refus intervient avant toute vérification OTP.
	require.Equal(t, 0, f.requests.RequestCount())
}

func TestCreateIndividualRequestOTPInvalideNeConsommeRien(t *testing.T) {
	f := newFixture()
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	seedOTP(t, f, "771000001", "999999") // code stocké différent

	_, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "OTP_INVALID", fault.Code)
	require.Equal(t, 0, f.requests.RequestCount())
}

func TestCreateIndividualRequestNumeroInconnu(t *testing.T) {
	f := newFixture()
	seedOTP(t, f, "771000001", "123456")
	// Aucun numero.Seed : le registre ne connaît pas ce numéro.

	_, fault := individualInteractor(f).Execute(ctxCaller(yasID), validIndividualInput("771000001"))
	require.NotNil(t, fault)
	require.Equal(t, "OPERATEUR_SOURCE_INCORRECT", fault.Code)
}

func TestCreateIndividualRequestDelaiPortageNonRespecte(t *testing.T) {
	f := newFixture()
	recent := time.Now().Add(-24 * time.Hour) // portée hier : bien sous 3 mois
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

// TestCreateIndividualRequestEchecEcritureNeConsommePasLOTP est la preuve,
// au niveau interactor, que l'OTP.Consume n'est jamais atteint quand l'un
// des appels d'écriture précédents échoue à l'intérieur de la même
// transaction — la garantie de commit 643415f. La preuve qu'un
// UnitOfWork RÉEL défait vraiment ses écritures sur ce même chemin vit dans
// internal/framework/persistence (Postgres, //go:build integration) : ce
// test-ci prouve l'ORDRE des appels, pas le rollback lui-même, que
// l'in-memory UnitOfWork ne peut pas simuler.
func TestCreateIndividualRequestEchecEcritureNeConsommePasLOTP(t *testing.T) {
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
	require.False(t, stored.Consumed, "l'OTP ne doit pas être consommé quand la création échoue")
}
