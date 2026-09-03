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

func newCORSHarness(t *testing.T) *corsHarness {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.AllowCORS())
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

// Every origin is allowed, with nothing to configure: a local double holds
// nothing worth protecting from a cross-origin read, and one variable fewer
// is one less reason for a browser call to fail unexplained.
func TestEveryOriginIsAllowed(t *testing.T) {
	h := newCORSHarness(t)

	for _, origin := range []string{"http://localhost:8081", "http://anywhere.example"} {
		resp := h.rawWithHeaders(http.MethodPost, "/api/authenticate",
			map[string]string{"Origin": origin})

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"), origin)
	}
}

// A call without an Origin is a server-to-server call: it must receive
// exactly what the real platform sends, which is no CORS header at all.
func TestNoOriginNoHeader(t *testing.T) {
	h := newCORSHarness(t)

	resp := h.rawWithHeaders(http.MethodPost, "/api/authenticate", nil)

	require.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

// The preflight goes out without an Authorization header: it must pass
// before the authentication middleware, or it gets refused with a 401 and
// the browser never issues the real request. DELETE is in the list for the
// sandbox purge, the one route of the surface that uses it.
func TestPreflightRespondsWithoutAuthentication(t *testing.T) {
	h := newCORSHarness(t)

	resp := h.rawWithHeaders(http.MethodOptions, "/api/gateway/v1/demandes/particulier",
		map[string]string{
			"Origin":                         "http://localhost:8081",
			"Access-Control-Request-Method":  "POST",
			"Access-Control-Request-Headers": "authorization,content-type",
		})

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "DELETE")
}

// Guards constraint D-4: CORS goes through a global middleware, not through
// registered OPTIONS routes — AllowCORS must never register a route on the
// engine that carries it.
func TestCORSAddsNoRoute(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.AllowCORS())

	require.Empty(t, r.Routes())
}
