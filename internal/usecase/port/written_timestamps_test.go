package port_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

func TestWrittenAtFindsTheValueTheRequestWrote(t *testing.T) {
	ctx := port.WithWrittenTimestamps(context.Background())
	written := time.Date(2026, 8, 27, 22, 39, 23, 583_043_149, time.UTC)
	port.MarkWritten(ctx, written)

	// Postgres returns the microsecond: the nanoseconds are lost.
	read := written.Truncate(time.Microsecond)
	got, ok := port.WrittenAt(ctx, read)
	require.True(t, ok)
	require.Equal(t, written, got)

	_, ok = port.WrittenAt(ctx, read.Add(time.Second))
	require.False(t, ok, "a timestamp from another request is not fresh")
}

func TestWithoutRegistryNothingIsWritten(t *testing.T) {
	ctx := context.Background()
	port.MarkWritten(ctx, time.Now()) // does not panic
	_, ok := port.WrittenAt(ctx, time.Now())
	require.False(t, ok)
}
