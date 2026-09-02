package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web"
)

// buildRouter mirrors cmd/server/main.go's own construction of web.NewRouter,
// minus a real database connection: NewRouter builds every controller
// eagerly (Task 9's build-once rationale), but building one issues no
// query — only a request that actually reaches postgres needs a real
// *persistence.DB — so an empty, unopened one is enough for these
// route-table and guard assertions, exactly as internal/api/cors_test.go's
// own TestLeCORSNAjouteAucuneRoute (deleted this task) relied on.
func buildRouter(adjust ...func(*config.Config)) *gin.Engine {
	cfg := &config.Config{Port: "0", JWTSecret: "s"}
	for _, f := range adjust {
		f(cfg)
	}
	gin.SetMode(gin.ReleaseMode)
	return web.NewRouter(&web.Deps{Cfg: cfg, DB: &persistence.DB{}})
}

// Guard on constraint D-4: CORS goes through a global middleware, not
// through registered OPTIONS routes, and the sandbox exposes exactly the
// contract's surface — no health, metrics or debug route. Moved from
// internal/api/cors_test.go (Task 18): that file counted routes on
// internal/api.NewRouter, the only test in the whole suite that ever
// asserted a route count; this is the same assertion, unchanged, against
// web.NewRouter now that it is the router actually served.
func TestCORSAddsNoRoute(t *testing.T) {
	without := len(buildRouter().Routes())
	with := len(buildRouter(func(c *config.Config) {
		c.CORSAllowedOrigins = []string{"*"}
	}).Routes())

	require.Equal(t, 35, without, "33 gateway routes plus the two authentication ones")
	require.Equal(t, without, with, "CORS must register no route")
}

// Guard on the same constraint, with SANDBOX_ADMIN on: one more route
// appears — DELETE /api/sandbox/v1/demandes — and none other.
func TestWithSandboxAdminOneMoreRoute(t *testing.T) {
	withSandbox := len(buildRouter(func(c *config.Config) {
		c.SandboxAdmin = true
	}).Routes())

	require.Equal(t, 36, withSandbox)
}

// Ruling R32: an unauthenticated request to an unknown path under the
// gateway prefix must answer 401, not 404 — the prefix guard's whole reason
// to exist as a global middleware rather than a group one (see
// GuardGatewayPrefix's own doc comment). Existing coverage handled a valid
// token on an unknown path (404: authentication passed, routing then found
// nothing — see conformite_captures_test.go and its siblings) and a known
// path without a token (401 — TestMissingTokenReturnsAccessForbiddenEnvelope
// and its neighbours, internal/framework/web/middleware). This is the missing
// combination: no token, unknown path, still 401 — added here, while the
// router that wires the guard globally is held.
func TestUnknownPathUnderGatewayWithoutTokenReturns401(t *testing.T) {
	r := buildRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, web.GatewayPrefix+"/route-inexistante", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// The sandbox surface's own asymmetry (constraint D-4's comment on
// NewRouter): an unknown path under /api/sandbox/v1 must answer 404, not
// 401 — that surface is not the platform's, and its group middleware only
// runs for routes actually registered in it.
func TestUnknownPathUnderSandboxReturns404(t *testing.T) {
	r := buildRouter(func(c *config.Config) { c.SandboxAdmin = true })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, web.SandboxPrefix+"/route-inexistante", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
