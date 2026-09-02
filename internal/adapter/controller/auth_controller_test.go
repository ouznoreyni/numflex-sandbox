package controller_test

// These 8 test functions are moved, unchanged in assertion, from the
// deleted internal/api/auth_test.go (Task 10). They still exercise the
// real, live router — routerharness.NewRouterHarness wraps api.NewRouter,
// wired exactly as cmd/server/main.go wires it — so a green run here proves
// a real HTTP request to /api/authenticate or /api/gateway/v1/... goes
// through the new AuthController and middleware.Authenticate, not through
// any leftover handler.
//
// This package (internal/adapter/controller) may not import
// internal/framework directly (test/architecture_test.go's dependency
// rule, which inspects test-file imports too); routerharness.RouterHarness
// is what lets this test drive the whole HTTP stack without doing so.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
)

func TestAuthenticationNominal(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	for _, c := range []struct{ user, pass string }{
		{"orange", "orange2026"},
		{"yas", "yas2026"},
		{"expresso", "expresso2026"},
	} {
		require.NotEmpty(t, h.Token(c.user, c.pass), c.user)
	}
}

func TestAuthenticationWrongPassword(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": "yas", "password": "faux", "rememberMe": false,
	})
	// ANO-016: an authentication failure comes out outside the envelope.
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.NotContains(t, body, "success")
	require.NotContains(t, body, "code")
}

func TestGetAuthenticateWithValidToken(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp := h.Raw(http.MethodGet, "/api/authenticate", h.Token("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestMissingTokenReturnsAccessForbiddenEnvelope(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodGet, "/api/gateway/v1/operateurs", "", nil)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, false, body["success"])
	require.Equal(t, "ACCES_INTERDIT", body["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		body["message"])
}

func TestInvalidTokenReturnsEmptyBodyWithoutContentType(t *testing.T) {
	// ANO-008: invalid token → 401, empty body, no Content-Type.
	h := routerharness.NewRouterHarness(t)
	resp := h.Raw(http.MethodGet, "/api/gateway/v1/operateurs", "token.bogus.xxx", nil)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, "", resp.Header.Get("Content-Type"))
	require.Equal(t, int64(0), resp.ContentLength)
}

func TestEmptyBearerTokenReturnsAccessForbiddenEnvelope(t *testing.T) {
	// The brief states: "no Authorization header, or an empty bearer".
	// "Bearer " (nothing after) must produce the enveloped 401 ACCES_INTERDIT,
	// not the empty-body 401 reserved for an invalid token.
	h := routerharness.NewRouterHarness(t)
	req, err := http.NewRequest(http.MethodGet, h.Srv.URL+"/api/gateway/v1/operateurs", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer ")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, false, body["success"])
	require.Equal(t, "ACCES_INTERDIT", body["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		body["message"])
}

func TestPrefixGuardDoesNotCaptureABorderingPath(t *testing.T) {
	// The prefix guard compares the whole segment: /api/gateway/v1extra is
	// not under the protected tree and must come out as 404, not 401, even
	// without a token — otherwise a future out-of-contract path
	// (/api/gateway/v10) would be wrongly absorbed by the authentication
	// middleware.
	h := routerharness.NewRouterHarness(t)
	resp := h.Raw(http.MethodGet, "/api/gateway/v1extra", "", nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnknownGatewayPathWithValidTokenStillReturns404(t *testing.T) {
	// Router scope guarantee: an authenticated request to a path not
	// declared under the gateway prefix must receive the ordinary 404, not
	// be absorbed by the authentication middleware.
	h := routerharness.NewRouterHarness(t)
	resp := h.Raw(http.MethodGet, "/api/gateway/v1/route-inexistante", h.Token("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
