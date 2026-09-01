package acceptance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/acceptance"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

const (
	orangeID = "operateur-orange"
	yasID    = "operateur-yas"

	motifID = "motif-identite-non-prouvee"
)

var errBoom = errors.New("échec simulé de la couche gateway")

func ctxCaller(operatorID string) context.Context {
	return port.WithCaller(context.Background(), entity.Caller{OperatorID: operatorID})
}

// fixture bundles the doubles an acceptance interactor test wires.
type fixture struct {
	requests *inmemory.RequestGateway
	reasons  *inmemory.ReferenceGateway
	uow      *inmemory.UnitOfWork
	engine   *inmemory.Engine
	clock    inmemory.FixedClock
}

func newFixture() *fixture {
	requests := inmemory.NewRequestGateway()
	return &fixture{
		requests: requests,
		reasons:  inmemory.NewReferenceGateway(),
		uow:      inmemory.NewUnitOfWork(port.Repositories{Requests: requests}),
		engine:   inmemory.NewEngine(),
		clock:    inmemory.FixedClock{At: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)},
	}
}

func acceptInteractor(f *fixture) *acceptance.AcceptRequestInteractor {
	return acceptance.NewAcceptRequest(f.requests, f.reasons, f.uow, f.engine, f.clock)
}

func seedRequest(f *fixture) entity.PortingRequest {
	pr := entity.PortingRequest{
		ID: "d1", RequestType: entity.RequestTypePorting, SubscriberType: entity.SubscriberIndividual,
		Status: entity.RequestInProgress, CurrentStep: entity.StepAcceptance,
		CurrentStepStatus:   entity.StepInProgress,
		SourceOperatorID:    orangeID,
		RecipientOperatorID: yasID,
		CreatorOperatorID:   yasID,
	}
	f.requests.Seed(pr)
	return pr
}

func TestAcceptRequestNominal(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	view, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true, Comment: "Demande conforme",
	})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)

	// La transition a été planifiée — preuve que le chemin d'acceptation
	// est bien allé jusqu'au bout.
	require.Equal(t, []string{"d1"}, f.engine.Scheduled)
	require.Equal(t, "Demande conforme", f.requests.Comment("d1"))
}

// TestAcceptRequestExecuteNeVerifiePlusLeGel documente le déplacement du
// fix round 1 : le gel de la place n'est plus l'affaire d'Execute — il doit
// être vérifié par AcceptanceController, AVANT le décodage du corps de la
// requête, pour que le cas « place gelée + corps invalide » réponde bien
// « place gelée » plutôt qu'une erreur de format JSON (voir
// TestAcceptationPlaceGeleePrimeSurCorpsInvalide au niveau HTTP). Un
// Execute qui vérifiait encore le gel après ce déplacement referait exactement
// l'erreur d'ordre que ce fix corrige : ce test le garantit en prouvant
// qu'une place gelée n'empêche plus Execute d'aboutir.
func TestAcceptRequestExecuteNeVerifiePlusLeGel(t *testing.T) {
	f := newFixture()
	seedRequest(f)
	f.engine.Frozen = true

	view, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true,
	})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
}

// TestMarketFrozen couvre directement la fonction exportée
// qu'AcceptanceController appelle désormais avant tout décodage — la seule
// vérification du gel qui subsiste, au niveau contrôleur plutôt qu'Execute.
func TestMarketFrozen(t *testing.T) {
	f := newFixture()
	require.Nil(t, acceptance.MarketFrozen(context.Background(), f.engine))

	f.engine.Frozen = true
	fault := acceptance.MarketFrozen(context.Background(), f.engine)
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)

	f.engine.Frozen = false
	f.engine.FailQuery = errBoom
	fault = acceptance.MarketFrozen(context.Background(), f.engine)
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
}

func TestAcceptRequestParLeDestinataireRefuse(t *testing.T) {
	// TC-034 : seul l'opérateur source peut décider.
	f := newFixture()
	seedRequest(f)

	_, fault := acceptInteractor(f).Execute(ctxCaller(yasID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true,
	})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
}

func TestAcceptRequestIdInconnu(t *testing.T) {
	f := newFixture()
	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "inconnu", Accept: true,
	})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

func TestAcceptRequestRejetSansMotifRefuse(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: false,
	})
	require.NotNil(t, fault)
	require.Equal(t, "MOTIF_REJET_OBLIGATOIRE", fault.Code)
}

func TestAcceptRequestRejetAvecMotifTermineLaDemande(t *testing.T) {
	f := newFixture()
	seedRequest(f)
	f.reasons.SeedRejectionReason(motifID, "Identité non prouvée")

	view, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: false, RejectionReasonID: motifID, Comment: "Contrat non résilié",
	})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
	require.Equal(t, entity.RequestRejected, f.requests.Status("d1"))
	require.Equal(t, motifID, f.requests.RejectionReason("d1"))
	require.Empty(t, f.engine.Scheduled, "un rejet ne planifie aucune transition")
}

func TestAcceptRequestMotifRejetInconnuRefuseMemeSurAcceptation(t *testing.T) {
	// Le contrôle d'existence du motifRejetId ne dépend pas de accepte : un
	// identifiant inconnu est refusé même sur une acceptation.
	f := newFixture()
	seedRequest(f)

	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true, RejectionReasonID: "inconnu-000",
	})
	require.NotNil(t, fault)
	require.Equal(t, "VALIDATION_ECHOUEE", fault.Code)
	require.Equal(t, "Motif de rejet inconnu", fault.Message)
}

// TestAcceptRequestEchecEcritureNArretePasAvantLaTransaction est la preuve,
// au niveau interactor, qu'un rejet dont l'écriture échoue ne va jamais
// jusqu'à planifier quoi que ce soit ni jusqu'à relire la demande — l'ordre
// des appels à l'intérieur de la même transaction. La preuve qu'un
// UnitOfWork RÉEL défait vraiment ses écritures sur ce même chemin (aucune
// ligne etape_historique, demande inchangée) vit dans
// internal/framework/persistence (Postgres, //go:build integration) : ce
// test-ci ne peut pas la simuler, un double en mémoire n'annulant rien.
func TestAcceptRequestEchecEcritureNArretePasAvantLaTransaction(t *testing.T) {
	f := newFixture()
	seedRequest(f)
	f.reasons.SeedRejectionReason(motifID, "Identité non prouvée")
	f.requests.FailReject = errBoom

	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: false, RejectionReasonID: motifID,
	})
	require.NotNil(t, fault)
	require.Equal(t, "ERREUR_INTERNE", fault.Code)
	require.NotEqual(t, entity.RequestRejected, f.requests.Status("d1"),
		"Reject a échoué : le statut ne doit pas avoir bougé")
}
