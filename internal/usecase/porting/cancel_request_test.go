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

// TestCancelRequestDelegueLAutorisationAEntityCanCancel documente que
// CancelRequestInteractor ne réimplémente aucune des deux règles
// d'entity.CanCancel : seul le créateur peut annuler, seule ACCEPTATION peut
// l'être.
func TestCancelRequestParLaSourceRefuse(t *testing.T) {
	f := newFixture()
	seedCancellableRequest(f)

	_, fault := cancelInteractor(f).Execute(ctxCaller(orangeID), "d1")
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.NotEqual(t, entity.RequestCancelled, f.requests.Status("d1"))
}

func TestCancelRequestApresAcceptationRefuse(t *testing.T) {
	f := newFixture()
	seedRequest(f) // DESACTIVATION par défaut : l'ACCEPTATION est déjà passée

	_, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "d1")
	require.NotNil(t, fault)
	require.Equal(t, "ETAPE_INVALIDE", fault.Code)
	require.Contains(t, fault.Message, "DESACTIVATION")
}

func TestCancelRequestIdInconnu(t *testing.T) {
	f := newFixture()
	_, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "inconnu")
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

// TestCancelRequestEchecEcritureNArretePasAvantLaTransaction est la preuve,
// au niveau interactor, qu'un échec d'écriture n'aboutit jamais à un statut
// modifié. La preuve qu'un UnitOfWork RÉEL défait vraiment ses deux
// écritures (etape_historique, demande) vit dans
// internal/framework/persistence (Postgres, //go:build integration) : un
// double en mémoire n'annule rien.
func TestCancelRequestEchecEcritureNArretePasAvantLaTransaction(t *testing.T) {
	f := newFixture()
	seedCancellableRequest(f)
	f.requests.FailCancel = errBoom

	_, fault := cancelInteractor(f).Execute(ctxCaller(yasID), "d1")
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.NotEqual(t, entity.RequestCancelled, f.requests.Status("d1"))
}
