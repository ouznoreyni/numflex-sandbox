package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/stretchr/testify/require"
)

// CORS is a sandbox convenience, not a trait of the contract: a gateway
// consumed server to server sends none, and no SIT measurement attests to it.
// The sandbox opens it to every origin by default, for comfort; on an empty
// list — CORS_ALLOWED_ORIGINS set to empty — the middleware goes silent again,
// and that is what this test checks.
func TestSansConfigurationAucunEnTeteCORS(t *testing.T) {
	h := nouveauHarnais(t)

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate", "",
		map[string]any{"username": "yas", "password": "yas2026"},
		map[string]string{"Origin": "http://localhost:8081"})

	require.Empty(t, rep.Header.Get("Access-Control-Allow-Origin"))
}

func TestOrigineAutoriseeRecoitLesEnTetes(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.CORSAllowedOrigins = []string{"http://localhost:8081"}
	})

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate", "",
		map[string]any{"username": "yas", "password": "yas2026"},
		map[string]string{"Origin": "http://localhost:8081"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	require.Equal(t, "http://localhost:8081", rep.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin", rep.Header.Get("Vary"))
}

func TestOrigineNonAutoriseeNeRecoitRien(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.CORSAllowedOrigins = []string{"http://localhost:8081"}
	})

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate", "",
		map[string]any{"username": "yas", "password": "yas2026"},
		map[string]string{"Origin": "http://ailleurs.example"})

	require.Empty(t, rep.Header.Get("Access-Control-Allow-Origin"))
}

// The preflight goes out without an Authorization header: it must pass before
// the authentication middleware, or it is refused with a 401 and the browser
// never sends the real request.
func TestPreambuleRepondSansAuthentification(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.CORSAllowedOrigins = []string{"http://localhost:8081"}
	})

	rep := h.brutAvecEnTetes(http.MethodOptions,
		"/api/gateway/v1/demandes/particulier", "", nil,
		map[string]string{
			"Origin":                         "http://localhost:8081",
			"Access-Control-Request-Method":  "POST",
			"Access-Control-Request-Headers": "authorization,content-type",
		})

	require.Equal(t, http.StatusNoContent, rep.StatusCode)
	require.Equal(t, "http://localhost:8081", rep.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, rep.Header.Get("Access-Control-Allow-Headers"), "Authorization")
	require.Contains(t, rep.Header.Get("Access-Control-Allow-Methods"), "POST")
}

// Guard on constraint D-4: CORS goes through a global middleware, not through
// registered OPTIONS routes. The exposed surface must not move.
func TestLeCORSNAjouteAucuneRoute(t *testing.T) {
	compter := func(ajuste ...func(*config.Config)) int {
		cfg := &config.Config{Port: "0", JWTSecret: "s"}
		for _, f := range ajuste {
			f(cfg)
		}
		gin.SetMode(gin.ReleaseMode)
		// A non-nil but empty DB: the test issues no query, yet NewRouter now
		// builds the OTP controller once, when the router is built — like
		// cmd/server/main.go, which always supplies a real *persistence.DB,
		// never nil.
		return len(NewRouter(&Deps{Cfg: cfg, DB: &persistence.DB{}}).Routes())
	}

	sans := compter()
	avec := compter(func(c *config.Config) {
		c.CORSAllowedOrigins = []string{"*"}
	})

	require.Equal(t, 35, sans, "33 routes gateway plus les deux d'authentification")
	require.Equal(t, sans, avec, "le CORS ne doit enregistrer aucune route")
}
