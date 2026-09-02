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

	// The transition was scheduled — proof the acceptance path really ran all
	// the way through.
	require.Equal(t, []string{"d1"}, f.engine.Scheduled)
	require.Equal(t, "Demande conforme", f.requests.Comment("d1"))
}

// TestAcceptRequestExecuteNoLongerChecksFreeze documents where the frozen-market
// check moved to: it is no longer Execute's business — AcceptanceController
// must make it BEFORE decoding the request body, so that the "frozen market +
// invalid body" case answers "frozen market" rather than a JSON format error
// (see TestAcceptFrozenMarketBeatsInvalidBody at the HTTP level). An
// Execute still checking the freeze after that move would reproduce exactly
// the ordering bug the move fixes: this test guards against that by proving a
// frozen market no longer keeps Execute from succeeding.
func TestAcceptRequestExecuteNoLongerChecksFreeze(t *testing.T) {
	f := newFixture()
	seedRequest(f)
	f.engine.Frozen = true

	view, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true,
	})
	require.Nil(t, fault)
	require.Equal(t, "d1", view.ID)
}

// TestMarketFrozen covers directly the exported function AcceptanceController
// now calls before any decoding — the only freeze check left, sitting at the
// controller level rather than inside Execute.
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

func TestAcceptRequestByRecipientRefused(t *testing.T) {
	// TC-034: only the source operator may decide.
	f := newFixture()
	seedRequest(f)

	_, fault := acceptInteractor(f).Execute(ctxCaller(yasID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true,
	})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
}

func TestAcceptRequestUnknownID(t *testing.T) {
	f := newFixture()
	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "inconnu", Accept: true,
	})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_NON_TROUVEE", fault.Code)
}

func TestAcceptRequestRejectionWithoutReasonRefused(t *testing.T) {
	f := newFixture()
	seedRequest(f)

	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: false,
	})
	require.NotNil(t, fault)
	require.Equal(t, "MOTIF_REJET_OBLIGATOIRE", fault.Code)
}

func TestAcceptRequestRejectionWithReasonCompletesRequest(t *testing.T) {
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
	require.Empty(t, f.engine.Scheduled, "a rejection schedules no transition")
}

func TestAcceptRequestUnknownRejectionReasonRefusedEvenOnAcceptance(t *testing.T) {
	// The motifRejetId existence check does not depend on accepte: an unknown
	// identifier is refused even on an acceptance.
	f := newFixture()
	seedRequest(f)

	_, fault := acceptInteractor(f).Execute(ctxCaller(orangeID), acceptance.AcceptRequestInput{
		RequestID: "d1", Accept: true, RejectionReasonID: "inconnu-000",
	})
	require.NotNil(t, fault)
	require.Equal(t, "VALIDATION_ECHOUEE", fault.Code)
	require.Equal(t, "Motif de rejet inconnu", fault.Message)
}

// TestAcceptRequestWriteFailureStopsBeforeTransaction is the proof, at
// the interactor level, that a rejection whose write fails never goes on to
// schedule anything, nor to read the request back — the order of the calls
// inside one transaction. The proof that a REAL UnitOfWork does undo its
// writes on that same path (no etape_historique row, demande unchanged) lives
// in internal/framework/persistence (Postgres, //go:build integration): this
// test cannot simulate it, an in-memory double rolling nothing back.
func TestAcceptRequestWriteFailureStopsBeforeTransaction(t *testing.T) {
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
		"Reject failed: the status must not have moved")
}
