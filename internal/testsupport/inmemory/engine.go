package inmemory

import (
	"context"
	"sync"
)

// Engine is a map-backed port.Engine, for use case unit tests that must not
// touch a database. Task 14 (acceptance) is its first caller.
type Engine struct {
	mu        sync.Mutex
	Frozen    bool  // MarketFrozen returns this, unconditionally
	FailQuery error // makes MarketFrozen return this error instead

	Scheduled    []string // every requestID ScheduleTransition was called with
	FailSchedule error
}

// NewEngine returns a double that answers "not frozen" until told otherwise.
func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) MarketFrozen(_ context.Context) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.FailQuery != nil {
		return false, e.FailQuery
	}
	return e.Frozen, nil
}

func (e *Engine) ScheduleTransition(_ context.Context, requestID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.FailSchedule != nil {
		return e.FailSchedule
	}
	e.Scheduled = append(e.Scheduled, requestID)
	return nil
}
