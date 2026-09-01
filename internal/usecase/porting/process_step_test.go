package porting_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/porting"
)

func processInteractor(f *fixture, completionLatency time.Duration) *porting.ProcessStepInteractor {
	return porting.NewProcessStep(f.requests, f.uow, f.engine, completionLatency)
}

func TestProcessStepNominal(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	view, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{
		RequestID: "d1", Comment: "Numéro désactivé",
	})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
	require.Equal(t, []string{"d1"}, f.engine.Scheduled,
		"la transition a été planifiée — preuve que le chemin va jusqu'au bout")
	require.Equal(t, "Numéro désactivé", f.requests.Comment("d1"))
}

func TestProcessStepSansCommentaireNEcritRien(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Empty(t, f.requests.Comment("d1"))
}

// TestProcessStepDelegueLAutorisationAEntityCanProcess documente que
// ProcessStepInteractor ne réimplémente aucune des règles de
// entity.CanProcess — TC-036 dans son principe : l'étape n'incombe pas à
// l'appelant.
func TestProcessStepDelegueLAutorisationAEntityCanProcess(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	_, fault := processInteractor(f, 0).Execute(ctxCaller(yasID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
}

func TestProcessStepRefuseAcceptationEtConfirmation(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) { pr.CurrentStep = entity.StepAcceptance })

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
}

func TestProcessStepIdInconnu(t *testing.T) {
	f := newFixture()
	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "inconnu"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

// TestProcessStepCompletionReverseReserveeALARTP documente que le refus
// d'une COMPLETION REVERSE — réservée à l'ARTP — vient d'entity.CanProcess,
// pas d'une condition locale à l'interactor.
func TestProcessStepCompletionReverseReserveeALARTP(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) {
		pr.RequestType = entity.RequestTypeReverse
		pr.CurrentStep = entity.StepCompletion
	})

	_, fault := processInteractor(f, 0).Execute(ctxCaller(yasID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Contains(t, fault.Message, "ARTP")
}

// TestProcessStepSecondAppelPendantLaConvergenceRefuse documente qu'un appel
// sur une demande dont la transition est déjà planifiée (PendingTransition)
// est refusé par entity.CanProcess.
func TestProcessStepSecondAppelPendantLaConvergenceRefuse(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) { pr.PendingTransition = true })

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
}

// TestProcessStepLatenceDeCompletion : ANO-005 — la seule étape lente.
func TestProcessStepLatenceDeCompletion(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) {
		pr.CurrentStep = entity.StepCompletion
		pr.SourceOperatorID, pr.RecipientOperatorID = orangeID, yasID
	})

	debut := time.Now()
	_, fault := processInteractor(f, 50*time.Millisecond).Execute(ctxCaller(yasID),
		porting.ProcessStepInput{RequestID: "d1"})
	ecoule := time.Since(debut)

	require.Nil(t, fault)
	require.GreaterOrEqual(t, ecoule, 50*time.Millisecond)
}

// TestProcessStepPasDeLatenceHorsCompletion documente que la latence ne
// s'applique qu'à COMPLETION, jamais à une autre étape.
func TestProcessStepPasDeLatenceHorsCompletion(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	debut := time.Now()
	_, fault := processInteractor(f, 50*time.Millisecond).Execute(ctxCaller(orangeID),
		porting.ProcessStepInput{RequestID: "d1"})
	ecoule := time.Since(debut)

	require.Nil(t, fault)
	require.Less(t, ecoule, 50*time.Millisecond)
}

// TestProcessStepEchecEcritureDuCommentaireNArretePasAvantLaTransaction est
// la preuve, au niveau interactor, qu'un échec d'écriture du commentaire ne
// va jamais jusqu'à planifier de transition. La preuve qu'un UnitOfWork RÉEL
// défait vraiment cette écriture vit dans internal/framework/persistence
// (Postgres, //go:build integration) : ce test-ci ne peut pas la simuler, un
// double en mémoire n'annulant rien.
func TestProcessStepEchecEcritureDuCommentaireNArretePasAvantLaTransaction(t *testing.T) {
	f := newFixture()
	seedRequest(f)
	f.requests.FailSetComment = errBoom

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{
		RequestID: "d1", Comment: "Numéro désactivé",
	})
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.Empty(t, f.engine.Scheduled,
		"un échec d'écriture du commentaire ne doit jamais aboutir à une transition planifiée")
}
