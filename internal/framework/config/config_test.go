package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	// make test exports the CI profile for the whole suite; this test is
	// about the default values, so it neutralizes those variables.
	for _, key := range []string{
		"STEP_TIMEOUT_SECONDS", "CONVERGENCE_MIN_SECONDS", "CONVERGENCE_MAX_SECONDS",
		"COMPLETION_LATENCY_MS", "CLOCK_SKEW_SECONDS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://x")

	c, err := Load()
	require.NoError(t, err)

	require.Equal(t, "8080", c.Port)
	require.Equal(t, FidelityReal, c.Fidelity)
	require.Equal(t, 349*time.Second, c.StepTimeout)
	require.Equal(t, 10*time.Second, c.EngineTick)
	// Zero convergence by default: the transition applies within the
	// request, as the 2026-08-27 captures show it. A value > 0 restores the
	// deferred behaviour measured at SIT v0.3 (R-10).
	require.Equal(t, time.Duration(0), c.ConvergenceMin)
	require.Equal(t, time.Duration(0), c.ConvergenceMax)
	// A hundred thousand per range: enough that no exploration exhausts it,
	// small enough that a container without a volume starts in seconds.
	require.Equal(t, DefaultPoolPerOperator, c.PoolPerOperator)
	require.Equal(t, 800_000, c.PoolPerOperator)
	require.False(t, c.FullNumbers)
	require.Equal(t, 30500*time.Millisecond, c.CompletionLatency)
	require.Equal(t, 540*time.Second, c.ClockSkew)
	require.Equal(t, "123456", c.OTPStaticCode)
	require.Equal(t, 300*time.Second, c.OTPTTL)
	require.Equal(t, 3, c.OTPMaxAttempts)
	require.Equal(t, 24*time.Hour, c.JWTTTL)
	require.Equal(t, time.Duration(0), c.ReverseAutoValidation)
}

// FULL_NUMBERS fills every portable range whole. It moves the pool's
// DEFAULT, so an explicit POOL_NUMBERS_PER_OPERATOR still wins and the two
// can never contradict each other.
func TestLoadFullNumbers(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")

	t.Setenv("FULL_NUMBERS", "true")
	c, err := Load()
	require.NoError(t, err)
	require.True(t, c.FullNumbers)
	require.Equal(t, 8_000_000, c.PoolPerOperator)

	t.Setenv("POOL_NUMBERS_PER_OPERATOR", "80000")
	c, err = Load()
	require.NoError(t, err)
	require.Equal(t, 80000, c.PoolPerOperator, "an explicit pool wins over the shortcut")
}

func TestLoadPoolBounds(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")

	// One number per range is the floor: below it a range would be empty.
	t.Setenv("POOL_NUMBERS_PER_OPERATOR", "7")
	_, err := Load()
	require.ErrorContains(t, err, "POOL_NUMBERS_PER_OPERATOR")

	// The ceiling is the six-digit tail: beyond it lpad would truncate and
	// numbers would collide.
	t.Setenv("POOL_NUMBERS_PER_OPERATOR", "8000001")
	_, err = Load()
	require.ErrorContains(t, err, "POOL_NUMBERS_PER_OPERATOR")

	t.Setenv("POOL_NUMBERS_PER_OPERATOR", "80000")
	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, 80000, c.PoolPerOperator)
}

func TestLoadCIProfile(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("STEP_TIMEOUT_SECONDS", "0")
	t.Setenv("CONVERGENCE_MIN_SECONDS", "0")
	t.Setenv("CONVERGENCE_MAX_SECONDS", "0")
	t.Setenv("COMPLETION_LATENCY_MS", "0")
	t.Setenv("CLOCK_SKEW_SECONDS", "0")

	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), c.StepTimeout)
	require.Equal(t, time.Duration(0), c.CompletionLatency)
	require.Equal(t, time.Duration(0), c.ClockSkew)
}

func TestLoadInvalidFidelity(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("FIDELITY", "presque")

	_, err := Load()
	require.Error(t, err)
}

func TestLoadDatabaseURLRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadInconsistentConvergence(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CONVERGENCE_MIN_SECONDS", "300")
	t.Setenv("CONVERGENCE_MAX_SECONDS", "60")

	_, err := Load()
	require.Error(t, err)
}
