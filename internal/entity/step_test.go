package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	orange   = "6a21745ce6c37b5b5b487ec1"
	yas      = "6a2174c3e6c37b5b5b487ec4"
	expresso = "6a217510e6c37b5b5b487ec7"
)

var place = []string{orange, yas, expresso}

func portingRequest() PortingRequest {
	return PortingRequest{
		ID: "d1", RequestType: RequestTypePorting, SubscriberType: SubscriberIndividual,
		Status: RequestInProgress, CurrentStep: StepAcceptance,
		CurrentStepStatus: StepInProgress,
		SourceOperatorID:  orange, RecipientOperatorID: yas,
		CreatorOperatorID: yas,
	}
}

func TestStepSequence(t *testing.T) {
	suite := []Step{
		StepAcceptance, StepDeactivation, StepActivation,
		StepConfirmation, StepCompletion,
	}
	for i := 0; i < len(suite)-1; i++ {
		suivante, ok := NextStep(suite[i])
		require.True(t, ok, string(suite[i]))
		require.Equal(t, suite[i+1], suivante)
	}
	_, ok := NextStep(StepCompletion)
	require.False(t, ok, "COMPLETION est terminale")
}

func TestStepOwner(t *testing.T) {
	require.Equal(t, RoleSource, StepOwner(StepAcceptance, RequestTypePorting))
	require.Equal(t, RoleSource, StepOwner(StepDeactivation, RequestTypePorting))
	require.Equal(t, RoleRecipient, StepOwner(StepActivation, RequestTypePorting))
	require.Equal(t, RoleAll, StepOwner(StepConfirmation, RequestTypePorting))
	require.Equal(t, RoleRecipient, StepOwner(StepCompletion, RequestTypePorting))

	// La COMPLETION d'un REVERSE est réservée à l'ARTP.
	require.Equal(t, RoleARTP, StepOwner(StepCompletion, RequestTypeReverse))
	require.Equal(t, RoleRecipient, StepOwner(StepCompletion, RequestTypeRestitution))
}

func TestCanProcessRefusesStepNotOwed(t *testing.T) {
	r := portingRequest()
	r.CurrentStep = StepDeactivation

	require.Nil(t, CanProcess(r, orange), "la source traite la DESACTIVATION")

	e := CanProcess(r, yas)
	require.NotNil(t, e, "le destinataire ne peut pas désactiver")
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
}

func TestCanProcessRefusesAcceptanceAndConfirmation(t *testing.T) {
	r := portingRequest()

	e := CanProcess(r, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"L'étape ACCEPTATION se traite via POST /api/gateway/v1/demandes/acceptation.",
		e.Message)

	r.CurrentStep = StepConfirmation
	e = CanProcess(r, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		e.Message)
}

func TestCanProcessRefusesReverseCompletion(t *testing.T) {
	r := portingRequest()
	r.RequestType = RequestTypeReverse
	r.CurrentStep = StepCompletion

	e := CanProcess(r, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
	require.Equal(t,
		"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.",
		e.Message)
}

func TestCanProcessRefusesRequestNotInProgress(t *testing.T) {
	r := portingRequest()
	r.CurrentStep = StepDeactivation
	r.Status = RequestCancelled

	e := CanProcess(r, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestCanProcessRefusesDuringConvergence(t *testing.T) {
	// R-10 : l'étape a été traitée, la transition n'est pas encore appliquée.
	r := portingRequest()
	r.CurrentStep = StepDeactivation
	r.PendingTransition = true

	e := CanProcess(r, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestExpectedConfirmersPortingExcludesRecipient(t *testing.T) {
	// D-6, mesuré au SIT : sur un portage ORANGE → YAS, EXPRESSO doit confirmer.
	r := portingRequest()
	r.CurrentStep = StepConfirmation

	require.ElementsMatch(t, []string{orange, expresso}, ExpectedConfirmers(r, place))
}

func TestExpectedConfirmersRestitutionAndReverseIncludeEveryone(t *testing.T) {
	for _, rt := range []RequestType{RequestTypeRestitution, RequestTypeReverse} {
		r := portingRequest()
		r.RequestType = rt
		r.CurrentStep = StepConfirmation
		require.ElementsMatchf(t, place, ExpectedConfirmers(r, place), string(rt))
	}
}

func TestCanAccept(t *testing.T) {
	r := portingRequest()

	require.Nil(t, CanAccept(r, orange))

	// TC-034 : le destinataire ne peut pas accepter sa propre demande.
	e := CanAccept(r, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)

	// Un tiers non plus.
	require.NotNil(t, CanAccept(r, expresso))

	// Hors de l'étape ACCEPTATION.
	r2 := portingRequest()
	r2.CurrentStep = StepActivation
	e = CanAccept(r2, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestCanCancel(t *testing.T) {
	r := portingRequest()

	require.Nil(t, CanCancel(r, yas), "le créateur annule")

	e := CanCancel(r, orange)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
	require.Equal(t,
		"Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		e.Message)

	r.CurrentStep = StepDeactivation
	e = CanCancel(r, yas)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		e.Message)
}

func TestErrorsAreFaults(t *testing.T) {
	var e *Fault = CanCancel(portingRequest(), orange)
	require.NotNil(t, e)
}
