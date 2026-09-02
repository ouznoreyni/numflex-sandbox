package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/acceptance"
)

func fleetInteractor(f *fixture) *acceptance.AcceptFleetRequestInteractor {
	return acceptance.NewAcceptFleetRequest(f.requests, f.reasons, f.uow, f.engine, f.clock)
}

func seedFleetRequest(f *fixture, numbers ...string) entity.PortingRequest {
	pr := entity.PortingRequest{
		ID: "flotte1", RequestType: entity.RequestTypePorting, SubscriberType: entity.SubscriberEnterprise,
		Status: entity.RequestInProgress, CurrentStep: entity.StepAcceptance,
		CurrentStepStatus:   entity.StepInProgress,
		SourceOperatorID:    orangeID,
		RecipientOperatorID: yasID,
		CreatorOperatorID:   yasID,
	}
	f.requests.Seed(pr)
	f.requests.SeedNumbers(pr.ID, numbers...)
	return pr
}

func TestAcceptFleetRequestWithPartialRejection(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002", "771000003")
	f.reasons.SeedRejectionReason(reasonID, "Numéro Inactif")

	view, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{
			{MSISDN: "771000002", RejectionReasonID: reasonID},
		},
		Comment: "Numéro 771000002 non conforme",
	})
	require.Nil(t, fault)
	require.Equal(t, "flotte1", view.ID)

	require.Equal(t, "REJETE", f.requests.NumberStatus("flotte1", "771000002"))
	require.Equal(t, reasonID, f.requests.NumberRejectionReason("flotte1", "771000002"))
	require.Equal(t, "EN_COURS", f.requests.NumberStatus("flotte1", "771000001"))
	require.NotEqual(t, entity.RequestRejected, f.requests.Status("flotte1"))
	require.Equal(t, []string{"flotte1"}, f.engine.Scheduled,
		"a still-active fleet schedules its transition")
}

func TestAcceptFleetRequestTotalRejection(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")
	f.reasons.SeedRejectionReason(reasonID, "Données manquantes")

	view, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: false, RejectionReasonID: reasonID,
		Comment: "Dossier incomplet",
	})
	require.Nil(t, fault)
	require.Equal(t, "flotte1", view.ID)
	require.Equal(t, entity.RequestRejected, f.requests.Status("flotte1"))
	require.Empty(t, f.engine.Scheduled)
}

// TestAcceptFleetRequestExhaustedNumberByNumberFlips is the proof of the
// [HYP] documented on accept_fleet_request.go: rejecting a fleet's numbers one
// by one until none is left flips the request itself to REJETE, with no
// transition scheduled — the same outcome an explicit total rejection has.
func TestAcceptFleetRequestExhaustedNumberByNumberFlips(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")
	f.reasons.SeedRejectionReason(reasonID, "Numéro Inactif")

	view, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{
			{MSISDN: "771000001", RejectionReasonID: reasonID},
			{MSISDN: "771000002", RejectionReasonID: reasonID},
		},
	})
	require.Nil(t, fault)
	require.Equal(t, "flotte1", view.ID)
	require.Equal(t, entity.RequestRejected, f.requests.Status("flotte1"))
	require.Empty(t, f.engine.Scheduled, "an exhausted fleet schedules no transition")
}

func TestAcceptFleetRequestNumberOutsideFleetRefused(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")

	_, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{{MSISDN: "771009999"}},
	})
	require.NotNil(t, fault)
	require.Equal(t, "VALIDATION_ECHOUEE", fault.Code)
	require.Contains(t, fault.Message, "771009999")

	// Nothing was written: validation failed before the transaction opened.
	require.Equal(t, "EN_COURS", f.requests.NumberStatus("flotte1", "771000001"))
	require.Equal(t, "EN_COURS", f.requests.NumberStatus("flotte1", "771000002"))
}

// TestAcceptFleetRequestWriteFailureStopsBeforeTransaction proves, at
// the interactor level, that a failure inside the transaction stops everything
// that follows — HasActiveNumber, Reject and the transition scheduling are
// never reached. See accept_request_test.go for the same proof on the
// individual side, and internal/framework/persistence for the proof of the
// real rollback against Postgres.
func TestAcceptFleetRequestWriteFailureStopsBeforeTransaction(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")
	f.requests.FailRejectNumber = errBoom

	_, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{{MSISDN: "771000001"}},
	})
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
	require.NotEqual(t, entity.RequestRejected, f.requests.Status("flotte1"))
}
