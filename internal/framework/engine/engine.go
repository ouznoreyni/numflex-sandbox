// Package engine is the platform engine's framework home: the ticker and its
// loop, and the two operations (PlaceGelee, PlanifierTransition) the rest of
// the sandbox calls synchronously between ticks — port.Engine's real
// implementation. The three behaviours a tick actually performs — expiring
// overdue steps (ANO-006), applying due convergences (R-10) and the reverse
// lifecycle reserved to the ARTP (§6) — are internal/usecase/platform
// interactors; this package only owns their construction and their fixed
// order inside Tick. ValiderReverse and RejeterReverse stay here as
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
// cmd/server/main.go and cmd/artp/main.go (through ValiderReverse and
// RejeterReverse) have always used.
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
		expire:   platform.NewExpireOverdueSteps(requests, uow, clk, cfg.EtapeTimeout),
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

// Tick effectue un passage : convergences dues, expirations, actes de l'ARTP.
func (e *Engine) Tick(ctx context.Context) error {
	// Un instant unique pour tout le tick : la convergence remet
	// date_debut_etape à now(), et si l'expiration recalculait son propre
	// now() ensuite, une demande qui vient de converger avec un EtapeTimeout
	// court pourrait à nouveau correspondre au prédicat d'expiration dans le
	// même passage.
	debutTick := time.Now()

	gelee, err := e.PlaceGelee(ctx)
	if err != nil {
		return err
	}
	if gelee {
		return nil
	}
	if err := e.converge.Execute(ctx); err != nil {
		return err
	}
	if err := e.expire.Execute(ctx, debutTick); err != nil {
		return err
	}
	return e.reverses.Execute(ctx)
}

// PlaceGelee : un incident de type figeSysteme ouvert, chez n'importe quel
// opérateur, bloque le traitement pour tout le monde (BR-012) — port.Engine's
// own MarketFrozen, delegated to port.IncidentGateway.MarketFrozen: see that
// method's own doc comment for why the read lives there rather than on a
// dedicated port.
func (e *Engine) PlaceGelee(ctx context.Context) (bool, error) {
	return e.incidents.MarketFrozen(ctx)
}

// PlanifierTransition marque l'étape courante comme traitée et fixe la date à
// laquelle la transition sera réellement appliquée. Entre les deux, la demande
// continue de présenter l'étape précédente — c'est le comportement mesuré (R-10).
// port.Engine's own ScheduleTransition.
func (e *Engine) PlanifierTransition(ctx context.Context, demandeID string) error {
	// Fenêtre nulle : la transition s'applique dans la requête, de sorte que le
	// DTO relu par le handler porte l'étape SUIVANTE. C'est ce que rendent les
	// captures du 2026-08-27 — /acceptation répond DESACTIVATION, /traitement
	// sur DESACTIVATION répond ACTIVATION, la dernière confirmation répond
	// COMPLETION — et c'est le défaut.
	//
	// Fenêtre non nulle : la transition est planifiée et le moteur l'applique
	// plus tard ; la réponse porte alors l'étape précédente. C'est le
	// comportement mesuré au SIT v0.3 (R-10), conservé pour qui doit éprouver
	// son intégration contre cette version-là de la plateforme.
	if e.cfg.ConvergenceMax <= 0 {
		return platform.ApplyTransition(ctx, e.uow, e.clock, demandeID, "ACTION")
	}

	delai := e.cfg.ConvergenceMin
	if ecart := e.cfg.ConvergenceMax - e.cfg.ConvergenceMin; ecart > 0 {
		delai += time.Duration(rand.Int63n(int64(ecart)))
	}
	return e.uow.Do(ctx, func(repos port.Repositories) error {
		return repos.Requests.ScheduleTransitionAt(ctx, demandeID, delai.Seconds())
	})
}

// ValiderReverse est un acte de l'ARTP, hors périmètre de l'API gateway
// (§6) : cmd/artp l'appelle directement contre un *persistence.DB, sans
// passer par un *Engine.
func ValiderReverse(ctx context.Context, db *persistence.DB, reverseID string) error {
	uow := persistence.NewUnitOfWork(db)
	return platform.ValidateReverse(ctx, uow, identifier.NewGenerator(), clock.New(0), reverseID)
}

// RejeterReverse est également un acte de l'ARTP, appelé directement par
// cmd/artp.
func RejeterReverse(ctx context.Context, db *persistence.DB, reverseID string) error {
	uow := persistence.NewUnitOfWork(db)
	return platform.RejectReverse(ctx, uow, reverseID)
}
