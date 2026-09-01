package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
)

// corsHarnais mounts a minimal engine carrying only AllowCORS and the two
// routes these tests exercise — not the full router, which does not exist
// at this level of the framework layer yet (Task 18 assembles it).
type corsHarnais struct {
	t   *testing.T
	srv *httptest.Server
}

func nouveauHarnaisCORS(t *testing.T, origines []string) *corsHarnais {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.AllowCORS(origines))
	r.POST("/api/authenticate", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/gateway/v1/demandes/particulier", func(c *gin.Context) { c.Status(http.StatusOK) })
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &corsHarnais{t: t, srv: srv}
}

func (h *corsHarnais) brutAvecEnTetes(methode, chemin string, entetes map[string]string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(methode, h.srv.URL+chemin, nil)
	require.NoError(h.t, err)
	for k, v := range entetes {
		req.Header.Set(k, v)
	}
	rep, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { rep.Body.Close() })
	return rep
}

// CORS is a sandbox convenience, not a trait of the contract: a gateway
// consumed server-to-server does not send it, and no SIT measurement
// attests to it. The sandbox opens it to every origin by default, for
// comfort; an empty list — origins set empty — makes the middleware mute
// again, which is what this test checks.
func TestSansConfigurationAucunEnTeteCORS(t *testing.T) {
	h := nouveauHarnaisCORS(t, nil)

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://localhost:8081"})

	require.Empty(t, rep.Header.Get("Access-Control-Allow-Origin"))
}

func TestOrigineAutoriseeRecoitLesEnTetes(t *testing.T) {
	h := nouveauHarnaisCORS(t, []string{"http://localhost:8081"})

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://localhost:8081"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	require.Equal(t, "http://localhost:8081", rep.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin", rep.Header.Get("Vary"))
}

func TestOrigineNonAutoriseeNeRecoitRien(t *testing.T) {
	h := nouveauHarnaisCORS(t, []string{"http://localhost:8081"})

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://ailleurs.example"})

	require.Empty(t, rep.Header.Get("Access-Control-Allow-Origin"))
}

func TestDefautToutesOriginesAutorisees(t *testing.T) {
	// config.Config's default is []string{"*"} — this test exercises that
	// value directly, the way the middleware actually receives it.
	h := nouveauHarnaisCORS(t, []string{"*"})

	rep := h.brutAvecEnTetes(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://n-importe-ou.example"})

	require.Equal(t, "*", rep.Header.Get("Access-Control-Allow-Origin"))
}

// Le préambule part sans en-tête Authorization : il doit passer avant le
// middleware d'authentification, sinon il est refusé en 401 et le
// navigateur n'émet jamais la vraie requête.
func TestPreambuleRepondSansAuthentification(t *testing.T) {
	h := nouveauHarnaisCORS(t, []string{"http://localhost:8081"})

	rep := h.brutAvecEnTetes(http.MethodOptions, "/api/gateway/v1/demandes/particulier",
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

// Garde-fou de la contrainte D-4 : le CORS passe par un middleware global,
// pas par des routes OPTIONS enregistrées — AllowCORS ne doit jamais
// enregistrer de route sur le moteur qui le porte.
func TestLeCORSNAjouteAucuneRoute(t *testing.T) {
	compter := func(origines []string) int {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(middleware.AllowCORS(origines))
		return len(r.Routes())
	}

	require.Equal(t, 0, compter(nil))
	require.Equal(t, 0, compter([]string{"*"}))
}
