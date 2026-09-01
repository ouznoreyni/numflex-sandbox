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

func TestAcceptFleetRequestAvecRejetPartiel(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002", "771000003")
	f.reasons.SeedRejectionReason(motifID, "Numéro Inactif")

	view, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{
			{MSISDN: "771000002", RejectionReasonID: motifID},
		},
		Comment: "Numéro 771000002 non conforme",
	})
	require.Nil(t, fault)
	require.Equal(t, "flotte1", view.ID)

	require.Equal(t, "REJETE", f.requests.NumberStatus("flotte1", "771000002"))
	require.Equal(t, motifID, f.requests.NumberRejectionReason("flotte1", "771000002"))
	require.Equal(t, "EN_COURS", f.requests.NumberStatus("flotte1", "771000001"))
	require.NotEqual(t, entity.RequestRejected, f.requests.Status("flotte1"))
	require.Equal(t, []string{"flotte1"}, f.engine.Scheduled,
		"une flotte encore active planifie sa transition")
}

func TestAcceptFleetRequestRejetTotal(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")
	f.reasons.SeedRejectionReason(motifID, "Données manquantes")

	view, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: false, RejectionReasonID: motifID,
		Comment: "Dossier incomplet",
	})
	require.Nil(t, fault)
	require.Equal(t, "flotte1", view.ID)
	require.Equal(t, entity.RequestRejected, f.requests.Status("flotte1"))
	require.Empty(t, f.engine.Scheduled)
}

// TestAcceptFleetRequestEpuiseeNumeroParNumeroBascule est la preuve du [HYP]
// documenté sur accept_fleet_request.go : rejeter chaque numéro d'une flotte
// jusqu'à épuisement bascule la demande elle-même en REJETE, sans transition
// planifiée — le même dénouement qu'un rejet total explicite.
func TestAcceptFleetRequestEpuiseeNumeroParNumeroBascule(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")
	f.reasons.SeedRejectionReason(motifID, "Numéro Inactif")

	view, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{
			{MSISDN: "771000001", RejectionReasonID: motifID},
			{MSISDN: "771000002", RejectionReasonID: motifID},
		},
	})
	require.Nil(t, fault)
	require.Equal(t, "flotte1", view.ID)
	require.Equal(t, entity.RequestRejected, f.requests.Status("flotte1"))
	require.Empty(t, f.engine.Scheduled, "une flotte épuisée ne planifie aucune transition")
}

func TestAcceptFleetRequestNumeroHorsFlotteRefuse(t *testing.T) {
	f := newFixture()
	seedFleetRequest(f, "771000001", "771000002")

	_, fault := fleetInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptFleetRequestInput{
		RequestID: "flotte1", Accept: true,
		RejectedNumbers: []acceptance.RejectedNumberInput{{MSISDN: "771009999"}},
	})
	require.NotNil(t, fault)
	require.Equal(t, "VALIDATION_ECHOUEE", fault.Code)
	require.Contains(t, fault.Message, "771009999")

	// Rien n'a été écrit : la validation a échoué avant l'ouverture de la
	// transaction.
	require.Equal(t, "EN_COURS", f.requests.NumberStatus("flotte1", "771000001"))
	require.Equal(t, "EN_COURS", f.requests.NumberStatus("flotte1", "771000002"))
}

// TestAcceptFleetRequestEchecEcritureNArretePasAvantLaTransaction prouve, au
// niveau interactor, qu'un échec pendant la transaction empêche toute suite
// — HasActiveNumber, Reject et la planification de la transition ne sont
// jamais atteints. Voir accept_request_test.go pour la même preuve côté
// individuel, et internal/framework/persistence pour la preuve du rollback
// réel contre Postgres.
func TestAcceptFleetRequestEchecEcritureNArretePasAvantLaTransaction(t *testing.T) {
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
