// Package engine is the platform engine's framework home: the ticker and its
// loop, and the two operations (MarketFrozen, ScheduleTransition) the rest of
// the sandbox calls synchronously between ticks — port.Engine's real
// implementation. The three behaviours a tick actually performs — expiring
// overdue steps (ANO-006), applying due convergences (R-10) and the reverse
// lifecycle reserved to the ARTP (§6) — are internal/usecase/platform
// interactors; this package only owns their construction and their fixed
// order inside Tick. ValidateReverse and RejectReverse stay here as
// package-level functions, unchanged in shape, because cmd/artp calls them
// directly against a *persistence.DB rather than through an *Engine.
package engine

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/platform"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

type Engine struct {
	cfg       *config.Config
	incidents port.IncidentGateway
	uow       port.UnitOfWork
	clock     port.Clock

	converge *platform.ConvergePendingTransitionsInteractor
	expire   *platform.ExpireOverdueStepsInteractor
	reverses *platform.AutoValidateReversesInteractor
}

// New wires an Engine against a real Postgres database — the shape
// cmd/server/main.go and cmd/artp/main.go (through ValidateReverse and
// RejectReverse) have always used.
func New(cfg *config.Config, db *persistence.DB) *Engine {
	uow := persistence.NewUnitOfWork(db)
	clk := clock.New(0)
	ids := identifier.NewGenerator()
	requests := postgres.NewRequestGateway(db.Pool)
	incidents := postgres.NewIncidentGateway(db.Pool)
	reverse := postgres.NewReverseGateway(db.Pool)

	return &Engine{
		cfg:       cfg,
		incidents: incidents,
		uow:       uow,
		clock:     clk,

		converge: platform.NewConvergePendingTransitions(requests, uow, clk),
		expire:   platform.NewExpireOverdueSteps(requests, uow, clk, cfg.StepTimeout),
		reverses: platform.NewAutoValidateReverses(reverse, requests, uow, ids, clk, cfg.ReverseAutoValidation),
	}
}

func (e *Engine) Run(ctx context.Context) {
	t := time.NewTicker(e.cfg.EngineTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := e.Tick(ctx); err != nil {
				log.Printf("moteur : %v", err)
			}
		}
	}
}

// Tick runs one pass: due convergences, expirations, ARTP acts.
func (e *Engine) Tick(ctx context.Context) error {
	// One single instant for the whole tick: convergence resets
	// date_debut_etape to now(), and if expiration recomputed its own now()
	// afterwards, a request that just converged with a short StepTimeout
	// could match the expiration predicate again within the same pass.
	tickStart := time.Now()

	frozen, err := e.MarketFrozen(ctx)
	if err != nil {
		return err
	}
	if frozen {
		return nil
	}
	if err := e.converge.Execute(ctx); err != nil {
		return err
	}
	if err := e.expire.Execute(ctx, tickStart); err != nil {
		return err
	}
	return e.reverses.Execute(ctx)
}

// MarketFrozen: an open figeSysteme incident, at any operator, blocks
// processing for everyone (BR-012) — port.Engine's own MarketFrozen,
// delegated to port.IncidentGateway.MarketFrozen: see that method's own doc
// comment for why the read lives there rather than on a dedicated port.
func (e *Engine) MarketFrozen(ctx context.Context) (bool, error) {
	return e.incidents.MarketFrozen(ctx)
}

// ScheduleTransition marks the current step as processed and fixes the date
// at which the transition will actually apply. Between the two, the request
// keeps presenting the previous step — this is the measured behaviour
// (R-10). port.Engine's own ScheduleTransition.
func (e *Engine) ScheduleTransition(ctx context.Context, requestID string) error {
	// Zero window: the transition applies within the request, so the DTO the
	// handler reads back carries the NEXT step. This is what the 2026-08-27
	// captures show — /acceptation answers DESACTIVATION, /traitement on
	// DESACTIVATION answers ACTIVATION, the last confirmation answers
	// COMPLETION — and it is the default.
	//
	// Non-zero window: the transition is scheduled and the engine applies it
	// later; the response then carries the previous step. This is the
	// behaviour measured at SIT v0.3 (R-10), kept for whoever must test
	// their integration against that version of the platform.
	if e.cfg.ConvergenceMax <= 0 {
		return platform.ApplyTransition(ctx, e.uow, e.clock, requestID, "ACTION")
	}

	delay := e.cfg.ConvergenceMin
	if spread := e.cfg.ConvergenceMax - e.cfg.ConvergenceMin; spread > 0 {
		delay += time.Duration(rand.Int63n(int64(spread)))
	}
	return e.uow.Do(ctx, func(repos port.Repositories) error {
		return repos.Requests.ScheduleTransitionAt(ctx, requestID, delay.Seconds())
	})
}

// ValidateReverse is an ARTP act, outside the API gateway's scope (§6):
// cmd/artp calls it directly against a *persistence.DB, without going
// through an *Engine.
func ValidateReverse(ctx context.Context, db *persistence.DB, reverseID string) error {
	uow := persistence.NewUnitOfWork(db)
	return platform.ValidateReverse(ctx, uow, identifier.NewGenerator(), clock.New(0), reverseID)
}

// RejectReverse is likewise an ARTP act, called directly by cmd/artp.
func RejectReverse(ctx context.Context, db *persistence.DB, reverseID string) error {
	uow := persistence.NewUnitOfWork(db)
	return platform.RejectReverse(ctx, uow, reverseID)
}
