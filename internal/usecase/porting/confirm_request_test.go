package porting_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/porting"
)

func confirmInteractor(f *fixture) *porting.ConfirmRequestInteractor {
	return porting.NewConfirmRequest(f.requests, f.operators, f.confirmations, f.uow, f.engine, f.clock)
}

func seedConfirmationRequest(f *fixture, mutate ...func(*entity.PortingRequest)) entity.PortingRequest {
	all := append([]func(*entity.PortingRequest){
		func(pr *entity.PortingRequest) { pr.CurrentStep = entity.StepConfirmation },
	}, mutate...)
	return seedRequest(f, all...)
}

// TestConfirmRequestPortingRecipientIsAutoConfirmed is
// entity.ExpectedConfirmers' PORTAGE rule, exercised end to end: ORANGE
// (source) confirms, the step stays open (only one of two expected
// confirmers so far — YAS the recipient is not itself expected); EXPRESSO,
// the third market operator, settles it.
func TestConfirmRequestPortingRecipientIsAutoConfirmed(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")
	f.operators.SeedOperator(expressoID, "EXPRESSO")

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Empty(t, f.engine.Scheduled, "EXPRESSO's confirmation is still missing")

	view, fault := confirmInteractor(f).Execute(ctxCaller(expressoID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
	require.Equal(t, []string{"d1"}, f.engine.Scheduled,
		"the last expected confirmation schedules the transition")
}

// TestConfirmRequestPortingByRecipientRefused: YAS, recipient of a
// PORTAGE, is not an expected confirmer — entity.ExpectedConfirmers excludes
// it.
func TestConfirmRequestPortingByRecipientRefused(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")

	_, fault := confirmInteractor(f).Execute(ctxCaller(yasID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
}

// TestConfirmRequestRestitutionRequiresRecipient is
// entity.ExpectedConfirmers' RESTITUTION/REVERSE rule: everyone confirms,
// recipient included.
func TestConfirmRequestRestitutionRequiresRecipient(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f, func(pr *entity.PortingRequest) {
		pr.RequestType = entity.RequestTypeRestitution
	})
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Empty(t, f.engine.Scheduled, "YAS, the recipient, still needs to confirm")

	_, fault = confirmInteractor(f).Execute(ctxCaller(yasID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Equal(t, []string{"d1"}, f.engine.Scheduled)
}

func TestConfirmRequestDoubleConfirmationRefused(t *testing.T) {
	// TC-041: anti-replay.
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)

	_, fault = confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Contains(t, fault.Message, "déjà confirmé")
}

func TestConfirmRequestOutsideConfirmationStepRefused(t *testing.T) {
	f := newFixture()
	seedRequest(f) // DESACTIVATION, not CONFIRMATION

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
	require.Contains(t, fault.Message, "DESACTIVATION")
}

func TestConfirmRequestTransitionAlreadyScheduledRefused(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f, func(pr *entity.PortingRequest) { pr.PendingTransition = true })

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
	require.Contains(t, fault.Message, "déjà été soldée")
}

func TestConfirmRequestUnknownID(t *testing.T) {
	f := newFixture()
	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "inconnu"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

// TestConfirmRequestWriteFailureStopsBeforeTransaction is the proof, at
// the interactor level, that a confirmation write failure never goes as far
// as counting confirmations or scheduling a transition. The proof that a
// REAL UnitOfWork really undoes this write lives in
// internal/framework/persistence (Postgres, //go:build integration): an
// in-memory double cancels nothing.
func TestConfirmRequestWriteFailureStopsBeforeTransaction(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.confirmations.FailConfirm = errBoom

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
}
