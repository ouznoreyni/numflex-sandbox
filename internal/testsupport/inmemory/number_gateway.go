package inmemory

import (
	"context"
	"sync"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// NumberGateway is a map-backed port.NumberGateway, keyed by MSISDN, for use
// case unit tests that must not touch a database.
type NumberGateway struct {
	mu     sync.Mutex
	states map[string]entity.NumberState
}

// NewNumberGateway returns an empty double, ready to use.
func NewNumberGateway() *NumberGateway {
	return &NumberGateway{states: make(map[string]entity.NumberState)}
}

// Seed registers a number's state directly, bypassing any use-case logic —
// interactor tests use it to set up a starting registry state (already
// ported recently, already restituted, a request already in progress) that
// Execute itself would never produce.
func (g *NumberGateway) Seed(state entity.NumberState) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.states == nil {
		g.states = make(map[string]entity.NumberState)
	}
	g.states[state.MSISDN] = state
}

func (g *NumberGateway) State(_ context.Context, msisdn string) (entity.NumberState, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.states[msisdn]
	return s, ok, nil
}
