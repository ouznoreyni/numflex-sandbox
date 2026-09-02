package porting_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/porting"
)

func cancelInteractor(f *fixture) *porting.CancelRequestInteractor {
	return porting.NewCancelRequest(f.requests, f.uow, f.clock)
}

func seedCancellableRequest(f *fixture, mutate ...func(*entity.PortingRequest)) entity.PortingRequest {
	all := append([]func(*entity.PortingRequest){
		func(pr *entity.PortingRequest) { pr.CurrentStep = entity.StepAcceptance },
	}, mutate...)
	return seedRequest(f, all...)
}

func TestCancelRequestNominal(t *testing.T) {
	f := newFixture()
	seedCancellableRequest(f)

	view, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "d1")
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
	require.Equal(t, entity.RequestCancelled, f.requests.Status("d1"))
}

// TestCancelRequestDelegatesAuthorizationToEntityCanCancel documents that
// CancelRequestInteractor reimplements neither of entity.CanCancel's two
// rules: only the creator may cancel, only ACCEPTATION may be cancelled.
func TestCancelRequestBySourceRefused(t *testing.T) {
	f := newFixture()
	seedCancellableRequest(f)

	_, fault := cancelInteractor(f).Execute(ctxCaller(orangeID), "d1")
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.NotEqual(t, entity.RequestCancelled, f.requests.Status("d1"))
}

func TestCancelRequestAfterAcceptanceRefused(t *testing.T) {
	f := newFixture()
	seedRequest(f) // DESACTIVATION by default: ACCEPTATION has already passed

	_, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "d1")
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
	require.Contains(t, fault.Message, "DESACTIVATION")
}

func TestCancelRequestUnknownID(t *testing.T) {
	f := newFixture()
	_, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "inconnu")
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

// TestCancelRequestWriteFailureStopsBeforeTransaction is the proof, at the
// interactor level, that a write failure never results in a modified
// status. The proof that a REAL UnitOfWork really undoes both of its
// writes (etape_historique, demande) lives in
// internal/framework/persistence (Postgres, //go:build integration): an
// in-memory double cancels nothing.
func TestCancelRequestWriteFailureStopsBeforeTransaction(t *testing.T) {
	f := newFixture()
	seedCancellableRequest(f)
	f.requests.FailCancel = errBoom

	_, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "d1")
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.NotEqual(t, entity.RequestCancelled, f.requests.Status("d1"))
}
