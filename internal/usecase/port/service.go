package port

import (
	"context"
	"time"
)

// Clock is injected so that CLOCK_SKEW_SECONDS and test determinism have a
// single seam. Now is the real instant; Rendered applies the configured skew
// and is used only when a timestamp leaves through a presenter.
type Clock interface {
	Now() time.Time
	Rendered(t time.Time) time.Time
}

type IDGenerator interface {
	NewID() string
}

// Engine is the part of platform behaviour that calls do not drive.
type Engine interface {
	MarketFrozen(ctx context.Context) (bool, error)
	ScheduleTransition(ctx context.Context, requestID string) error
}
