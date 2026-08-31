package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefauts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")

	c, err := Load()
	require.NoError(t, err)

	require.Equal(t, "8080", c.Port)
	require.Equal(t, FidelityReal, c.Fidelity)
	require.Equal(t, 349*time.Second, c.EtapeTimeout)
	require.Equal(t, 10*time.Second, c.EngineTick)
	require.Equal(t, 60*time.Second, c.ConvergenceMin)
	require.Equal(t, 360*time.Second, c.ConvergenceMax)
	require.Equal(t, 30500*time.Millisecond, c.CompletionLatency)
	require.Equal(t, 540*time.Second, c.ClockSkew)
	require.Equal(t, "123456", c.OTPStaticCode)
	require.Equal(t, 300*time.Second, c.OTPTTL)
	require.Equal(t, 3, c.OTPMaxAttempts)
	require.Equal(t, 24*time.Hour, c.JWTTTL)
	require.Equal(t, time.Duration(0), c.ReverseAutoValidation)
}

func TestLoadProfilCI(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ETAPE_TIMEOUT_SECONDS", "0")
	t.Setenv("CONVERGENCE_MIN_SECONDS", "0")
	t.Setenv("CONVERGENCE_MAX_SECONDS", "0")
	t.Setenv("COMPLETION_LATENCY_MS", "0")
	t.Setenv("CLOCK_SKEW_SECONDS", "0")

	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), c.EtapeTimeout)
	require.Equal(t, time.Duration(0), c.CompletionLatency)
	require.Equal(t, time.Duration(0), c.ClockSkew)
}

func TestLoadFideliteInvalide(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("FIDELITY", "presque")

	_, err := Load()
	require.Error(t, err)
}

func TestLoadDatabaseURLObligatoire(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadConvergenceIncoherente(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CONVERGENCE_MIN_SECONDS", "300")
	t.Setenv("CONVERGENCE_MAX_SECONDS", "60")

	_, err := Load()
	require.Error(t, err)
}
