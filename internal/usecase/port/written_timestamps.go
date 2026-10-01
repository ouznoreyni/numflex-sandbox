package port

import (
	"context"
	"sync"
	"time"
)

// The platform renders a timestamp at a precision that depends on who wrote
// it. Two sets of captures agree (individual 2026-08-27, enterprise
// 2026-09-18):
//
//   - a timestamp the request has just written comes out to the nanosecond —
//     dateDemande on creation ("22:39:23.583043149Z"), dateFinalisation on
//     COMPLETION ("23:14:55.794666796Z"): the platform serializes the Java
//     object it still holds in memory;
//   - the same timestamp read back by a later request comes out to the
//     millisecond ("22:39:23.583Z"): it comes back from Mongo, which stores
//     nothing finer.
//
// The sandbox stores in Postgres, at the microsecond. To reproduce the
// nanosecond, every write records its Go value in the request's context
// (MarkWritten), and rendering finds it again on read-back (WrittenAt);
// anything not recorded is truncated to the millisecond.

type writtenTimestampsKey struct{}

type writtenTimestamps struct {
	mu      sync.Mutex
	written []time.Time
}

// WithWrittenTimestamps returns a context carrying an empty registry. The
// router installs one per request; outside a request — an engine tick — there
// is none, and MarkWritten does nothing.
func WithWrittenTimestamps(ctx context.Context) context.Context {
	return context.WithValue(ctx, writtenTimestampsKey{}, &writtenTimestamps{})
}

// MarkWritten records that a write of the current request has just used t.
func MarkWritten(ctx context.Context, t time.Time) {
	if r, ok := ctx.Value(writtenTimestampsKey{}).(*writtenTimestamps); ok {
		r.mu.Lock()
		r.written = append(r.written, t)
		r.mu.Unlock()
	}
}

// WrittenAt returns, for a timestamp read back from the database, the
// full-precision value the current request wrote — or false if the write
// came from an earlier request. Postgres keeps the microsecond: the match is
// made at that precision.
func WrittenAt(ctx context.Context, read time.Time) (time.Time, bool) {
	r, ok := ctx.Value(writtenTimestampsKey{}).(*writtenTimestamps)
	if !ok {
		return time.Time{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.written {
		if t.Truncate(time.Microsecond).Equal(read.Truncate(time.Microsecond)) {
			return t, true
		}
	}
	return time.Time{}, false
}

// NowWritten reads clock and records the instant as written by the current
// request — the one call every write that a response may render goes through.
func NowWritten(ctx context.Context, clock Clock) time.Time {
	now := clock.Now()
	MarkWritten(ctx, now)
	return now
}
