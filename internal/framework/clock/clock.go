// Package clock is the framework layer's production implementation of
// port.Clock (internal/usecase/port/service.go). internal/httpx.Renderer
// used to own this behaviour inline as its Skew method; System reproduces it
// exactly so a presenter built for the live router renders the same
// timestamps as before.
package clock

import (
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// System is the real-instant, real-skew port.Clock: Now reads the system
// clock; Rendered applies the configured server clock skew
// (CLOCK_SKEW_SECONDS) and truncates to the millisecond.
//
// The truncation matches internal/httpx.Renderer.Skew verbatim: it comes
// from the 2026-08-27 captures — the platform reads its documents back from
// Mongo, whose timestamp is millisecond-precision, and so renders e.g.
// "2026-08-27T22:39:23.583Z". Postgres stores at microsecond precision;
// without truncation the sandbox would render a more precise field than the
// original, and a client comparing timestamps in their exact form would see
// the difference.
type System struct {
	skew time.Duration
}

// New returns a System clock applying skew on Rendered.
func New(skew time.Duration) System {
	return System{skew: skew}
}

func (c System) Now() time.Time { return time.Now() }

func (c System) Rendered(t time.Time) time.Time {
	return t.Add(c.skew).Truncate(time.Millisecond)
}

var _ port.Clock = System{}
