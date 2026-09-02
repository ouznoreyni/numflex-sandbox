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
		"the transition was scheduled — proof the path runs all the way through")
	require.Equal(t, "Numéro désactivé", f.requests.Comment("d1"))
}

func TestProcessStepWithoutCommentWritesNothing(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "d1"})
	require.Nil(t, fault)
	require.Empty(t, f.requests.Comment("d1"))
}

// TestProcessStepDelegatesAuthorizationToEntityCanProcess documents that
// ProcessStepInteractor reimplements none of entity.CanProcess's rules —
// TC-036 in principle: the step is not the caller's to handle.
func TestProcessStepDelegatesAuthorizationToEntityCanProcess(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	_, fault := processInteractor(f, 0).Execute(ctxCaller(yasID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Empty(t, f.engine.Scheduled)
}

func TestProcessStepRefusesAcceptanceAndConfirmation(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) { pr.CurrentStep = entity.StepAcceptance })

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
}

func TestProcessStepUnknownID(t *testing.T) {
	f := newFixture()
	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "inconnu"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

// TestProcessStepCompletionReverseReservedToARTP documents that the refusal
// of a REVERSE COMPLETION — reserved to the ARTP — comes from
// entity.CanProcess, not a condition local to the interactor.
func TestProcessStepCompletionReverseReservedToARTP(t *testing.T) {
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

// TestProcessStepSecondCallDuringConvergenceRefused documents that a call
// on a request whose transition is already scheduled (PendingTransition)
// is refused by entity.CanProcess.
func TestProcessStepSecondCallDuringConvergenceRefused(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) { pr.PendingTransition = true })

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{RequestID: "d1"})
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
}

// TestProcessStepCompletionLatency: ANO-005 — the only slow step.
func TestProcessStepCompletionLatency(t *testing.T) {
	f := newFixture()
	seedRequest(f, func(pr *entity.PortingRequest) {
		pr.CurrentStep = entity.StepCompletion
		pr.SourceOperatorID, pr.RecipientOperatorID = orangeID, yasID
	})

	start := time.Now()
	_, fault := processInteractor(f, 50*time.Millisecond).Execute(ctxCaller(yasID),
		porting.ProcessStepInput{RequestID: "d1"})
	elapsed := time.Since(start)

	require.Nil(t, fault)
	require.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

// TestProcessStepNoLatencyOutsideCompletion documents that the latency
// applies only to COMPLETION, never to any other step.
func TestProcessStepNoLatencyOutsideCompletion(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	start := time.Now()
	_, fault := processInteractor(f, 50*time.Millisecond).Execute(ctxCaller(orangeID),
		porting.ProcessStepInput{RequestID: "d1"})
	elapsed := time.Since(start)

	require.Nil(t, fault)
	require.Less(t, elapsed, 50*time.Millisecond)
}

// TestProcessStepCommentWriteFailureStopsBeforeTransaction is the proof,
// at the interactor level, that a comment write failure never goes as far
// as scheduling a transition. The proof that a REAL UnitOfWork really
// undoes this write lives in internal/framework/persistence (Postgres,
// //go:build integration): this test cannot simulate it, an in-memory
// double cancelling nothing.
func TestProcessStepCommentWriteFailureStopsBeforeTransaction(t *testing.T) {
	f := newFixture()
	seedRequest(f)
	f.requests.FailSetComment = errBoom

	_, fault := processInteractor(f, 0).Execute(ctxCaller(orangeID), porting.ProcessStepInput{
		RequestID: "d1", Comment: "Numéro désactivé",
	})
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.Empty(t, f.engine.Scheduled,
		"a comment write failure must never result in a scheduled transition")
}
