package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web"
)

// guardedRouter mounts GuardGatewayPrefix ahead of a single declared route
// under the gateway prefix, with authenticate rejecting every request — so
// that a 401 in these tests can only come from the guard actually running.
func guardedRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	authenticate := func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
		c.Abort()
	}
	r.Use(web.GuardGatewayPrefix(authenticate))
	r.GET(web.GatewayPrefix, func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET(web.GatewayPrefix+"/operateurs", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestGuardGatewayPrefixCapturesTheExactPrefix(t *testing.T) {
	r := guardedRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, web.GatewayPrefix, nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGuardGatewayPrefixCapturesANestedPath(t *testing.T) {
	r := guardedRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, web.GatewayPrefix+"/operateurs", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestGuardGatewayPrefixCapturesAnUndeclaredNestedPath is the behaviour the
// guard exists for: a path under the prefix that no route declares still
// reaches the middleware and comes back 401, not a router 404 — mirroring a
// Spring Security filter, which runs before route resolution.
func TestGuardGatewayPrefixCapturesAnUndeclaredNestedPath(t *testing.T) {
	r := guardedRouter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, web.GatewayPrefix+"/route-inexistante", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestGuardGatewayPrefixDoesNotCaptureABorderingPath asserts the
// whole-segment comparison: a path that merely starts with the prefix
// string, without the "/" boundary, is not under it.
func TestGuardGatewayPrefixDoesNotCaptureABorderingPath(t *testing.T) {
	r := guardedRouter()
	r.GET(web.GatewayPrefix+"extra", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, web.GatewayPrefix+"extra", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestGuardGatewayPrefixLeavesOtherPathsAlone asserts a path outside the
// gateway tree entirely reaches its own handler unauthenticated.
func TestGuardGatewayPrefixLeavesOtherPathsAlone(t *testing.T) {
	r := guardedRouter()
	r.GET("/api/authenticate", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/authenticate", nil)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
