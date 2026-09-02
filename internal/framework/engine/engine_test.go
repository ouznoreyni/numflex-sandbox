package engine

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
	"github.com/stretchr/testify/require"
)

// insertRequest creates a request directly in the database, at the wanted step.
func insertRequest(t *testing.T, db *persistence.DB, id string, step entity.Step, stepAge time.Duration) {
	t.Helper()
	start := time.Now().Add(-stepAge)
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,'771000001','PARTICULIER','PORTAGE','EN_COURS',$2,'EN_COURS',
		         $3,$4,$4,'PREPAID','191',now(),$5)`,
		id, string(step), seed.OperatorOrangeID, seed.OperatorYASID, start)
	require.NoError(t, err)

	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut) VALUES ($1,'771000001','EN_COURS')`, id)
	require.NoError(t, err)
}

func requestState(t *testing.T, db *persistence.DB, id string) (step, stepStatus, requestStatus string) {
	t.Helper()
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT etape_actuelle, statut_etape_actuel, statut_demande FROM demande WHERE id = $1`, id).
		Scan(&step, &stepStatus, &requestStatus))
	return
}

func newTestEngine(t *testing.T, adjust ...func(*config.Config)) (*Engine, *persistence.DB) {
	t.Helper()
	db := testsupport.NewTestDB(t)
	cfg := &config.Config{EngineTick: time.Millisecond, StepTimeout: 0}
	for _, f := range adjust {
		f(cfg)
	}
	return New(cfg, db), db
}

func TestExpirationAdvancesWithoutAnyCall(t *testing.T) {
	// TC-062 / ANO-006: steps progress on their own.
	e, db := newTestEngine(t, func(c *config.Config) { c.StepTimeout = 2 * time.Second })
	insertRequest(t, db, "d1", entity.StepAcceptance, 3*time.Second)

	require.NoError(t, e.Tick(context.Background()))

	step, stepStatus, requestStatus := requestState(t, db, "d1")
	require.Equal(t, "DESACTIVATION", step)
	require.Equal(t, "EN_COURS", stepStatus)
	require.Equal(t, "EN_COURS", requestStatus)

	// The history keeps a trace of the expiration.
	var origin, status string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine, statut FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'ACCEPTATION'`).Scan(&origin, &status))
	require.Equal(t, "EXPIRATION", origin)
	require.Equal(t, "EXPIRE", status)
}

func TestExpirationDoesNotAdvanceBeforeDelay(t *testing.T) {
	e, db := newTestEngine(t, func(c *config.Config) { c.StepTimeout = time.Hour })
	insertRequest(t, db, "d1", entity.StepAcceptance, time.Minute)

	require.NoError(t, e.Tick(context.Background()))

	step, _, _ := requestState(t, db, "d1")
	require.Equal(t, "ACCEPTATION", step)
}

func TestExpirationDisabledWhenDelayIsZero(t *testing.T) {
	e, db := newTestEngine(t) // StepTimeout = 0
	insertRequest(t, db, "d1", entity.StepAcceptance, 48*time.Hour)

	require.NoError(t, e.Tick(context.Background()))

	step, _, _ := requestState(t, db, "d1")
	require.Equal(t, "ACCEPTATION", step)
}

func TestFullCycleByExpiration(t *testing.T) {
	// SIT porting #2: created, no call, TERMINE 29 minutes later.
	e, db := newTestEngine(t, func(c *config.Config) { c.StepTimeout = time.Nanosecond })
	insertRequest(t, db, "d1", entity.StepAcceptance, time.Second)

	for i := 0; i < 5; i++ {
		require.NoError(t, e.Tick(context.Background()))
	}

	step, stepStatus, requestStatus := requestState(t, db, "d1")
	require.Equal(t, "COMPLETION", step)
	require.Equal(t, "EXPIRE", stepStatus)
	require.Equal(t, "TERMINE", requestStatus)

	var completedAt *time.Time
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT date_finalisation FROM demande WHERE id = 'd1'`).Scan(&completedAt))
	require.NotNil(t, completedAt)

	// The number has actually changed operator in the national registry.
	var current string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = '771000001'`).Scan(&current))
	require.Equal(t, seed.OperatorYASID, current)
}

// Zero convergence window — the default: ScheduleTransition applies the
// transition within the call, so the handler reads back the next step.
func TestZeroWindowAppliesTransitionImmediately(t *testing.T) {
	e, db := newTestEngine(t)
	insertRequest(t, db, "d1", entity.StepDeactivation, time.Second)

	require.NoError(t, e.ScheduleTransition(context.Background(), "d1"))

	step, _, _ := requestState(t, db, "d1")
	require.Equal(t, "ACTIVATION", step, "la transition est appliquée sans attendre un tick")

	// Nothing is left for the engine to do.
	require.NoError(t, e.Tick(context.Background()))
	step, _, _ = requestState(t, db, "d1")
	require.Equal(t, "ACTIVATION", step)
}

// Non-zero window: the deferred behaviour measured at SIT v0.3 (R-10).
func TestNonZeroWindowSchedulesThenApplies(t *testing.T) {
	e, db := newTestEngine(t, func(c *config.Config) {
		c.ConvergenceMin = 0
		c.ConvergenceMax = time.Millisecond
	})
	insertRequest(t, db, "d1", entity.StepDeactivation, time.Second)

	require.NoError(t, e.ScheduleTransition(context.Background(), "d1"))

	// As long as the engine has not run, the step stays the previous one.
	step, _, _ := requestState(t, db, "d1")
	require.Equal(t, "DESACTIVATION", step)

	require.NoError(t, e.Tick(context.Background()))

	step, _, _ = requestState(t, db, "d1")
	require.Equal(t, "ACTIVATION", step)

	var origin string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'DESACTIVATION'`).Scan(&origin))
	require.Equal(t, "ACTION", origin)
}

func TestConvergenceRespectsDelay(t *testing.T) {
	e, db := newTestEngine(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	insertRequest(t, db, "d1", entity.StepDeactivation, time.Second)

	require.NoError(t, e.ScheduleTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	step, _, _ := requestState(t, db, "d1")
	require.Equal(t, "DESACTIVATION", step, "la transition n'est pas encore due")
}

func TestRoutingRecalculatedEnteringConfirmation(t *testing.T) {
	e, db := newTestEngine(t)
	insertRequest(t, db, "d1", entity.StepActivation, time.Second)

	require.NoError(t, e.ScheduleTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	var routing string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT routage_info FROM demande WHERE id = 'd1'`).Scan(&routing))
	require.Equal(t, "192", routing, "routage destinataire (YAS) pour un numéro porté")
}

func TestFrozenMarketSuspendsEngine(t *testing.T) {
	// BR-012: an internal incident freezes processing for everyone.
	e, db := newTestEngine(t, func(c *config.Config) { c.StepTimeout = time.Nanosecond })
	insertRequest(t, db, "d1", entity.StepAcceptance, time.Second)

	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO incident (id, operateur_id, type_incident_id, fige_systeme,
		                       description, statut, date_ouverture)
		 VALUES ('i1',$1,$2,true,'panne','EN_COURS',now())`,
		seed.OperatorExpressoID, seed.IncidentTypeTechnicalID)
	require.NoError(t, err)

	frozen, err := e.MarketFrozen(context.Background())
	require.NoError(t, err)
	require.True(t, frozen)

	require.NoError(t, e.Tick(context.Background()))

	step, _, _ := requestState(t, db, "d1")
	require.Equal(t, "ACCEPTATION", step, "le moteur ne doit rien avancer pendant le gel")
}

func TestGatewayIncidentDoesNotFreeze(t *testing.T) {
	e, db := newTestEngine(t)
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO incident (id, operateur_id, type_incident_id, fige_systeme,
		                       description, statut, date_ouverture)
		 VALUES ('i1',$1,$2,false,'timeout','EN_COURS',now())`,
		seed.OperatorYASID, seed.IncidentTypeGatewayID)
	require.NoError(t, err)

	frozen, err := e.MarketFrozen(context.Background())
	require.NoError(t, err)
	require.False(t, frozen)
}

func TestRegistryTransferExcludesExcludedAndRejectedNumbers(t *testing.T) {
	// A filter that vanished from transfererAuRegistre or recalculerRoutage
	// would transfer — or route to the receiving operator — a rejected
	// number: a serious defect in a portability system.
	e, db := newTestEngine(t)
	insertRequest(t, db, "d1", entity.StepActivation, time.Second)

	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut, exclu)
		 VALUES ('d1','771000002','EN_COURS', true)`)
	require.NoError(t, err)
	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut, exclu)
		 VALUES ('d1','771000003','REJETE', false)`)
	require.NoError(t, err)

	require.NoError(t, e.ScheduleTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	currentOperator := func(msisdn string) string {
		var op string
		require.NoError(t, db.Pool.QueryRow(context.Background(),
			`SELECT operateur_actuel_id FROM numero WHERE msisdn = $1`, msisdn).Scan(&op))
		return op
	}
	numberRouting := func(msisdn string) string {
		var routing string
		require.NoError(t, db.Pool.QueryRow(context.Background(),
			`SELECT routage_info FROM demande_numero WHERE demande_id = 'd1' AND numero = $1`, msisdn).Scan(&routing))
		return routing
	}

	// The normal number has indeed changed operator and carries the
	// receiving operator's prefix.
	require.Equal(t, seed.OperatorYASID, currentOperator("771000001"))
	require.Equal(t, "192", numberRouting("771000001"))

	// The excluded and the rejected numbers stay with the source operator
	// and carry its prefix.
	require.Equal(t, seed.OperatorOrangeID, currentOperator("771000002"), "un numéro exclu ne doit pas être transféré")
	require.Equal(t, "191", numberRouting("771000002"))

	require.Equal(t, seed.OperatorOrangeID, currentOperator("771000003"), "un numéro rejeté ne doit pas être transféré")
	require.Equal(t, "191", numberRouting("771000003"))
}

// TestCancelDuringOngoingConvergence pins the outcome of a race Task 17
// inherited rather than introduced: porting.CancelRequestInteractor reads a
// request (entity.CanCancel) outside any lock, before opening its own
// port.UnitOfWork.Do. Task 17b closes the gap this used to leave open:
// RequestGateway.Cancel now guards both of its writes on the demande still
// sitting at the step the caller authorized against, so a convergence that
// applies in the window between that read and Cancel's write loses the
// race instead of overwriting stale information. This test reproduces the
// worst-case interleaving deterministically — no goroutines needed, since
// the race is a read-then-write ordering problem, not a data race in the Go
// sense — by applying the due convergence first and only then issuing the
// same guarded write porting.CancelRequestInteractor.Execute would have
// issued from its own (by-then stale) read.
//
// The result, now that the guard is in place: Cancel refuses
// (port.ErrCancelStepChanged), and the request is left exactly as the
// convergence left it — EN_COURS at DESACTIVATION, not ANNULE — with no
// etape_historique row for DESACTIVATION, a step nobody ever processed.
func TestCancelDuringOngoingConvergence(t *testing.T) {
	e, db := newTestEngine(t)
	insertRequest(t, db, "d1", entity.StepAcceptance, time.Second)

	// An operator accepts the request with a non-zero convergence window:
	// the transition is scheduled, already due.
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE demande SET transition_prevue_a = now() - interval '1 second' WHERE id = 'd1'`)
	require.NoError(t, err)

	// CancelRequestInteractor would read the request here and find it still
	// at ACCEPTATION (entity.CanCancel would authorize it against that step)
	// — read reproduced implicitly: nothing about the state below has
	// changed yet.
	step, _, status := requestState(t, db, "d1")
	require.Equal(t, "ACCEPTATION", step)
	require.Equal(t, "EN_COURS", status)
	authorizedStep := entity.Step(step)

	// ... then, before Cancel writes, the engine converges the scheduled
	// transition.
	require.NoError(t, e.Tick(context.Background()))
	step, _, status = requestState(t, db, "d1")
	require.Equal(t, "DESACTIVATION", step, "convergence moved the request forward")
	require.Equal(t, "EN_COURS", status)

	// ... and only then does Cancel write, carrying the step it was
	// authorized against (ACCEPTATION, from the stale read above) — the
	// same call porting.CancelRequestInteractor.Execute would make through
	// port.UnitOfWork.Do, made directly here to isolate the race from the
	// authorization that precedes it. The guard on that step no longer
	// matches the request's actual current step, so Cancel refuses.
	gw := postgres.NewRequestGateway(db.Pool)
	err = gw.Cancel(context.Background(), "d1", seed.OperatorOrangeID, authorizedStep, time.Now())
	require.ErrorIs(t, err, port.ErrCancelStepChanged)

	step, stepStatus, requestStatus := requestState(t, db, "d1")
	require.Equal(t, "EN_COURS", requestStatus, "Cancel refused: the request keeps convergence's own state")
	require.Equal(t, "DESACTIVATION", step, "the step convergence left it at, untouched by the refused cancel")
	require.Equal(t, "EN_COURS", stepStatus)

	// No etape_historique row was written for DESACTIVATION — nobody ever
	// processed that step, and the refused Cancel must not fabricate one.
	var n int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'DESACTIVATION'`).Scan(&n))
	require.Equal(t, 0, n, "no history row for a step nobody processed")
}
