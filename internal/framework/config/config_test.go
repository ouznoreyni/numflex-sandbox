package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	// make test exports the CI profile for the whole suite; this test is
	// about the default values, so it neutralizes those variables.
	for _, key := range []string{
		"ETAPE_TIMEOUT_SECONDS", "CONVERGENCE_MIN_SECONDS", "CONVERGENCE_MAX_SECONDS",
		"COMPLETION_LATENCY_MS", "CLOCK_SKEW_SECONDS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://x")
	// Not in the loop above: for CORS, the empty string is not absence — it
	// switches the headers off. The default reads as the variable removed.
	// t.Setenv records it first so cleanup restores it.
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	require.NoError(t, os.Unsetenv("CORS_ALLOWED_ORIGINS"))

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
	require.Equal(t, 30500*time.Millisecond, c.CompletionLatency)
	require.Equal(t, 540*time.Second, c.ClockSkew)
	require.Equal(t, "123456", c.OTPStaticCode)
	require.Equal(t, 300*time.Second, c.OTPTTL)
	require.Equal(t, 3, c.OTPMaxAttempts)
	require.Equal(t, 24*time.Hour, c.JWTTTL)
	require.Equal(t, time.Duration(0), c.ReverseAutoValidation)
	// The sandbox is called from pages — Swagger, a back-office in
	// development — and must work with no configuration.
	require.Equal(t, []string{"*"}, c.CORSAllowedOrigins)
}

// Setting the variable to empty gives the sandbox the real gateway's
// silence, which emits no CORS header at all. It is the only way to switch
// it off.
func TestLoadCORSEmptyDisablesHeaders(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	c, err := Load()
	require.NoError(t, err)
	require.Empty(t, c.CORSAllowedOrigins)
}

func TestLoadCORSExplicitList(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:8081, http://localhost:4200")

	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"http://localhost:8081", "http://localhost:4200"}, c.CORSAllowedOrigins)
}

func TestLoadCIProfile(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ETAPE_TIMEOUT_SECONDS", "0")
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
