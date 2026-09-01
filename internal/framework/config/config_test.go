package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefauts(t *testing.T) {
	// make test exporte le profil CI pour toute la suite ; ce test-ci porte sur
	// les valeurs par défaut, il neutralise donc ces variables.
	for _, clef := range []string{
		"ETAPE_TIMEOUT_SECONDS", "CONVERGENCE_MIN_SECONDS", "CONVERGENCE_MAX_SECONDS",
		"COMPLETION_LATENCY_MS", "CLOCK_SKEW_SECONDS",
	} {
		t.Setenv(clef, "")
	}
	t.Setenv("DATABASE_URL", "postgres://x")
	// Pas dans la boucle ci-dessus : pour le CORS, la chaîne vide n'est pas
	// l'absence — elle éteint les en-têtes. Le défaut se lit variable retirée.
	// t.Setenv l'enregistre d'abord pour que le nettoyage la restaure.
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	require.NoError(t, os.Unsetenv("CORS_ALLOWED_ORIGINS"))

	c, err := Load()
	require.NoError(t, err)

	require.Equal(t, "8080", c.Port)
	require.Equal(t, FidelityReal, c.Fidelity)
	require.Equal(t, 349*time.Second, c.EtapeTimeout)
	require.Equal(t, 10*time.Second, c.EngineTick)
	// Convergence nulle par defaut : la transition s'applique dans la requete,
	// comme le rendent les captures du 2026-08-27. Une valeur > 0 restaure le
	// comportement differe mesure au SIT v0.3 (R-10).
	require.Equal(t, time.Duration(0), c.ConvergenceMin)
	require.Equal(t, time.Duration(0), c.ConvergenceMax)
	require.Equal(t, 30500*time.Millisecond, c.CompletionLatency)
	require.Equal(t, 540*time.Second, c.ClockSkew)
	require.Equal(t, "123456", c.OTPStaticCode)
	require.Equal(t, 300*time.Second, c.OTPTTL)
	require.Equal(t, 3, c.OTPMaxAttempts)
	require.Equal(t, 24*time.Hour, c.JWTTTL)
	require.Equal(t, time.Duration(0), c.ReverseAutoValidation)
	// Le bac à sable est appelé depuis des pages — Swagger, un back-office en
	// développement — et doit marcher sans configuration.
	require.Equal(t, []string{"*"}, c.CORSAllowedOrigins)
}

// Poser la variable à vide rend au sandbox le silence de la gateway réelle,
// qui n'émet aucun en-tête CORS. C'est la seule façon de l'éteindre.
func TestLoadCORSVideEteintLesEnTetes(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	c, err := Load()
	require.NoError(t, err)
	require.Empty(t, c.CORSAllowedOrigins)
}

func TestLoadCORSListeExplicite(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:8081, http://localhost:4200")

	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"http://localhost:8081", "http://localhost:4200"}, c.CORSAllowedOrigins)
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
