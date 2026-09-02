package inmemory

import (
	"fmt"
	"sync"
)

// IDGenerator is a port.IDGenerator double producing predictable, distinct
// ids ("id-1", "id-2", …) — deterministic, unlike identifier.Generator's
// real ObjectId-shaped output, which use-case tests have no reason to need.
type IDGenerator struct {
	mu   sync.Mutex
	next int
}

// NewIDGenerator returns a double starting at "id-1".
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

func (g *IDGenerator) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	return fmt.Sprintf("id-%d", g.next)
}
