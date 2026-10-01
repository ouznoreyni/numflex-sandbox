package controller

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

func TestFormatInstantFollowsJavaISOInstant(t *testing.T) {
	base := time.Date(2026, 8, 27, 22, 39, 23, 0, time.UTC)
	cases := []struct {
		ns   int
		want string
	}{
		{0, "2026-08-27T22:39:23Z"},
		{583_000_000, "2026-08-27T22:39:23.583Z"},
		{580_000_000, "2026-08-27T22:39:23.580Z"}, // not « .58Z »
		{583_043_000, "2026-08-27T22:39:23.583043Z"},
		{583_043_149, "2026-08-27T22:39:23.583043149Z"},
		{5_000, "2026-08-27T22:39:23.000005Z"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, formatInstant(base.Add(time.Duration(c.ns))))
	}
}

func TestFormatInstantConvertsToUTC(t *testing.T) {
	dakar := time.FixedZone("GMT+1", 3600)
	require.Equal(t, "2026-08-27T21:39:23.583Z",
		formatInstant(time.Date(2026, 8, 27, 22, 39, 23, 583_000_000, dakar)))
}

func TestInstantMarshalsAsString(t *testing.T) {
	b, err := json.Marshal(map[string]any{
		"d": instant(time.Date(2026, 8, 27, 22, 39, 23, 583_043_149, time.UTC)),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"d":"2026-08-27T22:39:23.583043149Z"}`, string(b))
}

// skewedClock renders as framework/clock.System does — skew, then
// truncation to the millisecond — without the adapter layer importing it.
type skewedClock struct{ skew time.Duration }

func (c skewedClock) Now() time.Time { return time.Now() }
func (c skewedClock) Rendered(t time.Time) time.Time {
	return t.Add(c.skew).Truncate(time.Millisecond)
}

// The skew (ANO-015) applies in both cases; only the precision differs.
func TestRenderedInstantPrecisionDependsOnWhoWrote(t *testing.T) {
	clk := skewedClock{skew: 9 * time.Minute}
	written := time.Date(2026, 8, 27, 22, 39, 23, 583_043_149, time.UTC)
	read := written.Truncate(time.Microsecond) // what Postgres gives back

	ctx := port.WithWrittenTimestamps(context.Background())
	port.MarkWritten(ctx, written)
	require.Equal(t, "2026-08-27T22:48:23.583043149Z",
		formatInstant(time.Time(renderedInstant(ctx, clk, read))))

	require.Equal(t, "2026-08-27T22:48:23.583Z",
		formatInstant(time.Time(renderedInstant(context.Background(), clk, read))))
}
