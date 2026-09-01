package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ConfirmationGateway is a map-backed port.ConfirmationGateway, for use case
// unit tests that must not touch a database. Confirm's replay guard mirrors
// the confirmation table's own primary key (demande_id, operateur_id):
// confirming twice from the same operator returns port.ErrAlreadyConfirmed,
// exactly as the Postgres gateway's 23505 translation does.
type ConfirmationGateway struct {
	mu        sync.Mutex
	confirmed map[string]map[string]bool // requestID -> operatorID -> confirmed

	FailConfirm error // fails every Confirm call, the seam porting's atomicity test uses
	FailCount   error
}

// NewConfirmationGateway returns an empty double, ready to use.
func NewConfirmationGateway() *ConfirmationGateway {
	return &ConfirmationGateway{confirmed: map[string]map[string]bool{}}
}

func (g *ConfirmationGateway) Confirm(_ context.Context, requestID, operatorID, _ string, _ time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailConfirm != nil {
		return g.FailConfirm
	}
	if g.confirmed[requestID] == nil {
		g.confirmed[requestID] = map[string]bool{}
	}
	if g.confirmed[requestID][operatorID] {
		return port.ErrAlreadyConfirmed
	}
	g.confirmed[requestID][operatorID] = true
	return nil
}

func (g *ConfirmationGateway) Count(_ context.Context, requestID string) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FailCount != nil {
		return 0, g.FailCount
	}
	return len(g.confirmed[requestID]), nil
}

// Confirmers exposes which operators have confirmed requestID — tests use it
// to assert Confirm actually recorded the right operator.
func (g *ConfirmationGateway) Confirmers(requestID string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, 0, len(g.confirmed[requestID]))
	for op := range g.confirmed[requestID] {
		out = append(out, op)
	}
	return out
}
