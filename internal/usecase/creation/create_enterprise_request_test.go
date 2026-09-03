package creation_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/creation"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
)

func enterpriseInteractor(f *fixture) *creation.CreateEnterpriseRequestInteractor {
	verify := otp.NewVerifyOTP(f.otp, f.clock, 3)
	return creation.NewCreateEnterpriseRequest(verify, f.numbers, f.uow, f.ids, f.clock)
}

func validEnterpriseInput(fleetMSISDN string, fleet []string) creation.CreateEnterpriseRequestInput {
	return creation.CreateEnterpriseRequestInput{
		FleetMSISDN: fleetMSISDN, OTPCode: "123456",
		SourceOperatorID: orangeID, RecipientOperatorID: yasID,
		Process: "POSTPAID", FleetNumbers: fleet,
		Client: creation.ClientInput{
			LastName: "Diallo", FirstName: "Ousmane", BirthDate: "1975-03-20",
			BirthPlace: "Dakar", IDType: "CNI", IDNumber: "123",
			CompanyName: "Entreprise SARL", RCNumber: "RC-1",
		},
	}
}

func TestCreateEnterpriseRequestNominal(t *testing.T) {
	f := newFixture()
	f.requests.SeedPrefix(orangeID, "191")
	for _, n := range []string{"771000001", "771000002", "771000003"} {
		f.numbers.Seed(entity.NumberState{MSISDN: n, CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	}
	seedOTP(t, f, "771000001", "123456")

	out, fault := enterpriseInteractor(f).Execute(ctxCaller(yasID),
		validEnterpriseInput("771000001", []string{"771000001", "771000002", "771000003"}))
	require.Nil(t, fault)
	require.Equal(t, 3, out.RetainedCount)
	require.Empty(t, out.Excluded)
	require.Len(t, f.requests.Numbers(out.ID), 3)

	stored, found, err := f.otp.Find(context.Background(), "771000001")
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, stored.Consumed)
}

func TestCreateEnterpriseRequestEmptyFleet(t *testing.T) {
	f := newFixture()

	_, fault := enterpriseInteractor(f).Execute(ctxCaller(yasID),
		validEnterpriseInput("771000001", []string{}))
	require.NotNil(t, fault)
	require.Equal(t, "FLOTTE_VIDE", fault.Code)
	require.Equal(t, 0, f.requests.RequestCount())
}

func TestCreateEnterpriseRequestMixedOperators(t *testing.T) {
	f := newFixture()
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	f.numbers.Seed(entity.NumberState{MSISDN: "701000001", CurrentOperatorID: "operateur-expresso", OriginOperatorID: "operateur-expresso"})
	seedOTP(t, f, "771000001", "123456")

	_, fault := enterpriseInteractor(f).Execute(ctxCaller(yasID),
		validEnterpriseInput("771000001", []string{"771000001", "701000001"}))
	require.NotNil(t, fault)
	require.Equal(t, "FLOTTE_OPERATEURS_MIXTES", fault.Code)
}

func TestCreateEnterpriseRequestPartialExclusion(t *testing.T) {
	// BR-006 / invariant 11: the fleet succeeds with fewer numbers than requested.
	f := newFixture()
	f.requests.SeedPrefix(orangeID, "191")
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})
	f.numbers.Seed(entity.NumberState{
		MSISDN: "771000002", CurrentOperatorID: orangeID, OriginOperatorID: orangeID,
		RequestInProgress: true, // already blocked by another request EN_COURS
	})
	seedOTP(t, f, "771000001", "123456")

	out, fault := enterpriseInteractor(f).Execute(ctxCaller(yasID),
		validEnterpriseInput("771000001", []string{"771000001", "771000002"}))
	require.Nil(t, fault)
	require.Equal(t, 1, out.RetainedCount)
	require.Len(t, out.Excluded, 1)
	require.Equal(t, "771000002", out.Excluded[0].MSISDN)
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", out.Excluded[0].ErrorCode)
	require.Len(t, f.requests.Excluded(out.ID), 1)
}

func TestCreateEnterpriseRequestNoEligibleNumber(t *testing.T) {
	f := newFixture()
	within3Months := time.Now().AddDate(0, 0, -25) // ported 25 days ago
	f.numbers.Seed(entity.NumberState{
		MSISDN: "779000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID,
		LastPortingDate: &within3Months,
	})
	seedOTP(t, f, "779000001", "123456")

	_, fault := enterpriseInteractor(f).Execute(ctxCaller(yasID),
		validEnterpriseInput("779000001", []string{"779000001"}))
	require.NotNil(t, fault)
	require.Equal(t, "AUCUN_NUMERO_ELIGIBLE", fault.Code)
	require.Equal(t, 0, f.requests.RequestCount())
}
