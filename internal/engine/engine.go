// Package engine reproduit ce que la plateforme NumFlex fait sans qu'aucun
// opérateur n'agisse : l'expiration des étapes (ANO-006), la convergence différée
// après traitement (R-10), et les actes réservés à l'ARTP.
package engine

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/store"
)

type Engine struct {
	cfg *config.Config
	db  *store.DB
}

func New(cfg *config.Config, db *store.DB) *Engine {
	return &Engine{cfg: cfg, db: db}
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
	gelee, err := e.PlaceGelee(ctx)
	if err != nil {
		return err
	}
	if gelee {
		return nil
	}
	if err := e.appliquerConvergencesDues(ctx); err != nil {
		return err
	}
	if err := e.expirerEtapes(ctx); err != nil {
		return err
	}
	if err := e.validerReversesAutomatiquement(ctx); err != nil {
		return err
	}
	return e.completerReversesConfirmes(ctx)
}

// PlaceGelee : un incident de type figeSysteme ouvert, chez n'importe quel
// opérateur, bloque le traitement pour tout le monde (BR-012).
func (e *Engine) PlaceGelee(ctx context.Context) (bool, error) {
	var n int
	err := e.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM incident i
		   JOIN type_incident t ON t.id = i.type_incident_id
		  WHERE i.statut = 'EN_COURS' AND t.fige_systeme`).Scan(&n)
	return n > 0, err
}

// PlanifierTransition marque l'étape courante comme traitée et fixe la date à
// laquelle la transition sera réellement appliquée. Entre les deux, la demande
// continue de présenter l'étape précédente — c'est le comportement mesuré (R-10).
func (e *Engine) PlanifierTransition(ctx context.Context, demandeID string) error {
	delai := e.cfg.ConvergenceMin
	if ecart := e.cfg.ConvergenceMax - e.cfg.ConvergenceMin; ecart > 0 {
		delai += time.Duration(rand.Int63n(int64(ecart)))
	}
	_, err := e.db.Pool.Exec(ctx,
		`UPDATE demande SET transition_prevue_a = $2 WHERE id = $1`,
		demandeID, time.Now().Add(delai))
	return err
}

func (e *Engine) appliquerConvergencesDues(ctx context.Context) error {
	rows, err := e.db.Pool.Query(ctx,
		`SELECT id FROM demande
		  WHERE statut_demande = 'EN_COURS'
		    AND transition_prevue_a IS NOT NULL
		    AND transition_prevue_a <= now()`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range ids {
		if err := e.AppliquerTransition(ctx, id, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) expirerEtapes(ctx context.Context) error {
	if e.cfg.EtapeTimeout <= 0 {
		return nil
	}
	rows, err := e.db.Pool.Query(ctx,
		`SELECT id FROM demande
		  WHERE statut_demande = 'EN_COURS'
		    AND transition_prevue_a IS NULL
		    AND date_debut_etape + make_interval(secs => $1) <= now()`,
		e.cfg.EtapeTimeout.Seconds())
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range ids {
		if err := e.AppliquerTransition(ctx, id, "EXPIRATION"); err != nil {
			return err
		}
	}
	return nil
}

// validerReversesAutomatiquement et completerReversesConfirmes sont implémentées
// en Task 19. Déclarées ici pour que Tick compile et reste complet.
func (e *Engine) validerReversesAutomatiquement(ctx context.Context) error {
	return nil
}

func (e *Engine) completerReversesConfirmes(ctx context.Context) error {
	return nil
}
