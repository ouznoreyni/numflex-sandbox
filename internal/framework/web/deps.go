package web

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
)

// Deps carries the pieces cmd/server/main.go (the composition root) already
// holds by the time it calls NewRouter — configuration, the opened
// database, and the running platform engine — plus the presenter fidelity
// choice, made once here rather than at every capability's own constructor.
// Moved from internal/api/deps.go (Task 18): this is the same struct,
// stripped of its dead R *httpx.Renderer field (nothing ever read it — every
// controller renders through internal/adapter/presenter instead) now that
// internal/httpx is deleted.
type Deps struct {
	Cfg    *config.Config
	DB     *persistence.DB
	Moteur Moteur
}

// Moteur — the part of the platform's behaviour that calls do not drive.
type Moteur interface {
	PlaceGelee(ctx context.Context) (bool, error)
	PlanifierTransition(ctx context.Context, demandeID string) error
}

// presenter picks Real or Contract according to the configured fidelity
// mode — every capability's own controller constructor calls this once.
func (d *Deps) presenter() presenter.Presenter {
	clk := clock.New(d.Cfg.ClockSkew)
	if d.Cfg.Fidelity == config.FidelityContract {
		return presenter.NewContract(clk)
	}
	return presenter.NewReal(clk)
}
