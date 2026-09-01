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

// TestConfirmRequestPortageLeDestinataireEstAutoConfirme is
// entity.ExpectedConfirmers' PORTAGE rule, exercised end to end: ORANGE
// (source) confirms, the step stays open (only one of two expected
// confirmers so far — YAS the recipient is not itself expected); EXPRESSO,
// the third market operator, settles it.
func TestConfirmRequestPortageLeDestinataireEstAutoConfirme(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")
	f.operators.SeedOperator(expressoID, "EXPRESSO")

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Empty(t, f.engine.Scheduled, "il manque encore la confirmation d'EXPRESSO")

	view, fault := confirmInteractor(f).Execute(ctxCaller(expressoID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
	require.Equal(t, []string{"d1"}, f.engine.Scheduled,
		"la dernière confirmation attendue planifie la transition")
}

// TestConfirmRequestPortageParLeDestinataireRefuse: YAS, recipient of a
// PORTAGE, is not an expected confirmer — entity.ExpectedConfirmers excludes
// it.
func TestConfirmRequestPortageParLeDestinataireRefuse(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")

	_, fault := confirmInteractor(f).Execute(ctxCaller(yasID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
}

// TestConfirmRequestRestitutionExigeLeDestinataire is
// entity.ExpectedConfirmers' RESTITUTION/REVERSE rule: everyone confirms,
// recipient included.
func TestConfirmRequestRestitutionExigeLeDestinataire(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f, func(pr *entity.PortingRequest) {
		pr.RequestType = entity.RequestTypeRestitution
	})
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.operators.SeedOperator(yasID, "YAS")

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Empty(t, f.engine.Scheduled, "YAS, destinataire, doit encore confirmer")

	_, fault = confirmInteractor(f).Execute(ctxCaller(yasID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Equal(t, []string{"d1"}, f.engine.Scheduled)
}

func TestConfirmRequestDoubleConfirmationRefusee(t *testing.T) {
	// TC-041 : anti-rejeu.
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

func TestConfirmRequestHorsEtapeConfirmationRefuse(t *testing.T) {
	f := newFixture()
	seedRequest(f) // DESACTIVATION, pas CONFIRMATION

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
	require.Contains(t, fault.Message, "DESACTIVATION")
}

func TestConfirmRequestTransitionDejaPlanifieeRefuse(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f, func(pr *entity.PortingRequest) { pr.PendingTransition = true })

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
	require.Contains(t, fault.Message, "déjà été soldée")
}

func TestConfirmRequestIdInconnu(t *testing.T) {
	f := newFixture()
	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "inconnu"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

// TestConfirmRequestEchecEcritureNArretePasAvantLaTransaction est la preuve,
// au niveau interactor, qu'un échec d'écriture de la confirmation ne va
// jamais jusqu'à compter les confirmations ni planifier de transition. La
// preuve qu'un UnitOfWork RÉEL défait vraiment cette écriture vit dans
// internal/framework/persistence (Postgres, //go:build integration) : un
// double en mémoire n'annule rien.
func TestConfirmRequestEchecEcritureNArretePasAvantLaTransaction(t *testing.T) {
	f := newFixture()
	seedConfirmationRequest(f)
	f.operators.SeedOperator(orangeID, "ORANGE")
	f.confirmations.FailConfirm = errBoom

	_, fault := confirmInteractor(f).Execute(ctxCaller(orangeID), porting.ConfirmRequestInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
}
