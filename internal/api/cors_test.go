package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/stretchr/testify/require"
)

// Le CORS est une commodité de bac à sable, pas un trait du contrat : une
// gateway consommée de serveur à serveur n'en envoie pas, et aucune mesure du
// SIT n'en atteste. Le sandbox l'ouvre à toute origine par défaut, pour le
// confort ; liste vide — CORS_ALLOWED_ORIGINS posée vide — le middleware
// redevient muet, et c'est ce que ce test vérifie.
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

// Le préambule part sans en-tête Authorization : il doit passer avant le
// middleware d'authentification, sinon il est refusé en 401 et le navigateur
// n'émet jamais la vraie requête.
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

// Garde-fou de la contrainte D-4 : le CORS passe par un middleware global, pas
// par des routes OPTIONS enregistrées. La surface exposée ne doit pas bouger.
func TestLeCORSNAjouteAucuneRoute(t *testing.T) {
	compter := func(ajuste ...func(*config.Config)) int {
		cfg := &config.Config{Port: "0", JWTSecret: "s"}
		for _, f := range ajuste {
			f(cfg)
		}
		gin.SetMode(gin.ReleaseMode)
		return len(NewRouter(&Deps{Cfg: cfg}).Routes())
	}

	sans := compter()
	avec := compter(func(c *config.Config) {
		c.CORSAllowedOrigins = []string{"*"}
	})

	require.Equal(t, 35, sans, "33 routes gateway plus les deux d'authentification")
	require.Equal(t, sans, avec, "le CORS ne doit enregistrer aucune route")
}
