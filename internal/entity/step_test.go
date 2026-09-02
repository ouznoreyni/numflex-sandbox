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

var operators = []string{orange, yas, expresso}

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
		next, ok := NextStep(suite[i])
		require.True(t, ok, string(suite[i]))
		require.Equal(t, suite[i+1], next)
	}
	_, ok := NextStep(StepCompletion)
	require.False(t, ok, "COMPLETION is terminal")
}

func TestStepOwner(t *testing.T) {
	require.Equal(t, RoleSource, StepOwner(StepAcceptance, RequestTypePorting))
	require.Equal(t, RoleSource, StepOwner(StepDeactivation, RequestTypePorting))
	require.Equal(t, RoleRecipient, StepOwner(StepActivation, RequestTypePorting))
	require.Equal(t, RoleAll, StepOwner(StepConfirmation, RequestTypePorting))
	require.Equal(t, RoleRecipient, StepOwner(StepCompletion, RequestTypePorting))

	// A REVERSE's COMPLETION is reserved to the ARTP.
	require.Equal(t, RoleARTP, StepOwner(StepCompletion, RequestTypeReverse))
	require.Equal(t, RoleRecipient, StepOwner(StepCompletion, RequestTypeRestitution))
}

func TestCanProcessRefusesStepNotOwed(t *testing.T) {
	r := portingRequest()
	r.CurrentStep = StepDeactivation

	require.Nil(t, CanProcess(r, orange), "the source handles DESACTIVATION")

	e := CanProcess(r, yas)
	require.NotNil(t, e, "the recipient cannot deactivate")
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
	// R-10: the step has been processed, the transition is not yet applied.
	r := portingRequest()
	r.CurrentStep = StepDeactivation
	r.PendingTransition = true

	e := CanProcess(r, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestExpectedConfirmersPortingExcludesRecipient(t *testing.T) {
	// D-6, measured at the SIT: on an ORANGE → YAS porting, EXPRESSO must confirm.
	r := portingRequest()
	r.CurrentStep = StepConfirmation

	require.ElementsMatch(t, []string{orange, expresso}, ExpectedConfirmers(r, operators))
}

func TestExpectedConfirmersRestitutionAndReverseIncludeEveryone(t *testing.T) {
	for _, rt := range []RequestType{RequestTypeRestitution, RequestTypeReverse} {
		r := portingRequest()
		r.RequestType = rt
		r.CurrentStep = StepConfirmation
		require.ElementsMatchf(t, operators, ExpectedConfirmers(r, operators), string(rt))
	}
}

func TestCanAccept(t *testing.T) {
	r := portingRequest()

	require.Nil(t, CanAccept(r, orange))

	// TC-034: the recipient cannot accept its own request.
	e := CanAccept(r, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)

	// A third party cannot either.
	require.NotNil(t, CanAccept(r, expresso))

	// Outside the ACCEPTATION step.
	r2 := portingRequest()
	r2.CurrentStep = StepActivation
	e = CanAccept(r2, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestCanCancel(t *testing.T) {
	r := portingRequest()

	require.Nil(t, CanCancel(r, yas), "the creator cancels")

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
