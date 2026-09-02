package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
)

// corsHarness mounts a minimal engine carrying only AllowCORS and the two
// routes these tests exercise — not the full router, which does not exist
// at this level of the framework layer yet (Task 18 assembles it).
type corsHarness struct {
	t   *testing.T
	srv *httptest.Server
}

func newCORSHarness(t *testing.T, origins []string) *corsHarness {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.AllowCORS(origins))
	r.POST("/api/authenticate", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/api/gateway/v1/demandes/particulier", func(c *gin.Context) { c.Status(http.StatusOK) })
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &corsHarness{t: t, srv: srv}
}

func (h *corsHarness) rawWithHeaders(method, path string, headers map[string]string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(method, h.srv.URL+path, nil)
	require.NoError(h.t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// CORS is a sandbox convenience, not a trait of the contract: a gateway
// consumed server-to-server does not send it, and no SIT measurement
// attests to it. The sandbox opens it to every origin by default, for
// comfort; an empty list — origins set empty — makes the middleware mute
// again, which is what this test checks.
func TestWithoutConfigurationNoCORSHeader(t *testing.T) {
	h := newCORSHarness(t, nil)

	resp := h.rawWithHeaders(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://localhost:8081"})

	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestAllowedOriginReceivesHeaders(t *testing.T) {
	h := newCORSHarness(t, []string{"http://localhost:8081"})

	resp := h.rawWithHeaders(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://localhost:8081"})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "http://localhost:8081", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Origin", resp.Header.Get("Vary"))
}

func TestDisallowedOriginReceivesNothing(t *testing.T) {
	h := newCORSHarness(t, []string{"http://localhost:8081"})

	resp := h.rawWithHeaders(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://elsewhere.example"})

	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestDefaultAllOriginsAllowed(t *testing.T) {
	// config.Config's default is []string{"*"} — this test exercises that
	// value directly, the way the middleware actually receives it.
	h := newCORSHarness(t, []string{"*"})

	resp := h.rawWithHeaders(http.MethodPost, "/api/authenticate",
		map[string]string{"Origin": "http://anywhere.example"})

	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

// The preflight goes out without an Authorization header: it must pass
// before the authentication middleware, or it gets refused with a 401 and
// the browser never issues the real request.
func TestPreflightRespondsWithoutAuthentication(t *testing.T) {
	h := newCORSHarness(t, []string{"http://localhost:8081"})

	resp := h.rawWithHeaders(http.MethodOptions, "/api/gateway/v1/demandes/particulier",
		map[string]string{
			"Origin":                         "http://localhost:8081",
			"Access-Control-Request-Method":  "POST",
			"Access-Control-Request-Headers": "authorization,content-type",
		})

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "http://localhost:8081", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
}

// Guards constraint D-4: CORS goes through a global middleware, not through
// registered OPTIONS routes — AllowCORS must never register a route on the
// engine that carries it.
func TestCORSAddsNoRoute(t *testing.T) {
	countRoutes := func(origins []string) int {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(middleware.AllowCORS(origins))
		return len(r.Routes())
	}

	require.Equal(t, 0, countRoutes(nil))
	require.Equal(t, 0, countRoutes([]string{"*"}))
}
