package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// instant serializes a time.Time as java.time.Instant.toString() does — the
// form every timestamp the platform renders takes.
type instant time.Time

func (i instant) MarshalJSON() ([]byte, error) {
	return []byte(`"` + formatInstant(time.Time(i)) + `"`), nil
}

// formatInstant reproduces java.time.format.DateTimeFormatter.ISO_INSTANT:
// UTC, and the fraction of a second in groups of three digits — none, three,
// six or nine — depending on the last significant digit. ".580Z", never
// ".58Z" as time.RFC3339Nano would write it.
func formatInstant(t time.Time) string {
	t = t.UTC()
	base := t.Format("2006-01-02T15:04:05")
	ns := t.Nanosecond()
	switch {
	case ns == 0:
		return base + "Z"
	case ns%1_000_000 == 0:
		return fmt.Sprintf("%s.%03dZ", base, ns/1_000_000)
	case ns%1_000 == 0:
		return fmt.Sprintf("%s.%06dZ", base, ns/1_000)
	default:
		return fmt.Sprintf("%s.%09dZ", base, ns)
	}
}

// renderedInstant renders a timestamp read back from the database as the
// platform does (see port.WrittenAt): the server clock skew (ANO-015) applies
// at render time only, and the precision is the nanosecond when the current
// request has just written the value, the clock's millisecond otherwise.
// The sub-millisecond part is re-added after clk.Rendered, which truncates.
func renderedInstant(ctx context.Context, clk port.Clock, t time.Time) instant {
	if written, ok := port.WrittenAt(ctx, t); ok {
		subMillisecond := written.Sub(written.Truncate(time.Millisecond))
		return instant(clk.Rendered(written).Truncate(time.Millisecond).Add(subMillisecond))
	}
	return instant(clk.Rendered(t))
}
