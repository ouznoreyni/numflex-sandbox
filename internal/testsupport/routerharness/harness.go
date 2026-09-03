// Package routerharness starts the real, live router for adapter-layer
// controller tests. It is a separate package from internal/testsupport
// (rather than living alongside NewTestDB there) specifically because it
// imports internal/framework/web: internal/testsupport is itself imported
// by internal/framework/web's own tests (for NewTestDB), and
// internal/framework/web importing back from the same package would be a
// test import cycle. Only controller test packages (internal/adapter/
// controller and its successors) import routerharness.
package routerharness

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/stretchr/testify/require"
)

// RouterHarness starts the real, live router — web.NewRouter, wired exactly
// as cmd/server/main.go wires it — against a fresh, migrated, seeded test
// database. It exists so an adapter-layer controller's test (which cannot
// import internal/framework directly: see test/architecture_test.go) can
// still exercise the whole HTTP stack end to end, proving a request really
// reaches its controller rather than some leftover handler.
type RouterHarness struct {
	T      *testing.T
	Srv    *httptest.Server
	DB     *persistence.DB
	Cfg    *config.Config
	Engine *engine.Engine
}

// ContractFidelity switches the harness's fidelity mode to "contract" — the
// adjustment TestCreateIndividualInContractModeReturnsABusinessCode and
// TestEmptyFleetReturnsFlotteVideCode need. It exists here, rather than being
// written inline as func(c *config.Config) { c.Fidelity = ... } at the call
// site, because a controller's own test package cannot import
// internal/framework/config directly (test/architecture_test.go's
// dependency rule applies to _test.go files too): routerharness already
// imports config for NewRouterHarness's own signature, so this is the one
// place that dependency is allowed to live.
func ContractFidelity(c *config.Config) {
	c.Fidelity = config.FidelityContract
}

// Convergence returns an adjust function that fixes the engine's
// convergence window to [min, max] — the same reason ContractFidelity
// exists: a controller test cannot name internal/framework/config's
// *config.Config field directly, only through a func literal built here.
// A non-zero window makes ScheduleTransition defer the transition (R-10)
// rather than apply it synchronously within the request.
func Convergence(min, max time.Duration) func(*config.Config) {
	return func(c *config.Config) {
		c.ConvergenceMin, c.ConvergenceMax = min, max
	}
}

// CompletionLatency returns an adjust function that fixes
// config.CompletionLatency (ANO-005) — the same reason ContractFidelity
// exists.
func CompletionLatency(d time.Duration) func(*config.Config) {
	return func(c *config.Config) { c.CompletionLatency = d }
}

// NewRouterHarness mounts the full router in a deterministic profile.
// adjust lets a test override a default config value before the router is
// built.
func NewRouterHarness(t *testing.T, adjust ...func(*config.Config)) *RouterHarness {
	t.Helper()
	db := testsupport.NewTestDB(t)

	cfg := &config.Config{
		Port:           "0",
		JWTSecret:      "test-secret",
		JWTTTL:         24 * time.Hour,
		Fidelity:       config.FidelityReal,
		EngineTick:     10 * time.Millisecond,
		OTPStaticCode:  "123456",
		OTPTTL:         5 * time.Minute,
		OTPMaxAttempts: 3,
	}
	for _, f := range adjust {
		f(cfg)
	}

	eng := engine.New(cfg, db)
	d := &web.Deps{
		Cfg:    cfg,
		DB:     db,
		Engine: eng,
	}
	srv := httptest.NewServer(web.NewRouter(d))
	t.Cleanup(srv.Close)

	return &RouterHarness{T: t, Srv: srv, DB: db, Cfg: cfg, Engine: eng}
}

// Converge triggers one engine pass and checks that no transition is still
// due. Tests drive the engine explicitly rather than waiting on its
// ticker — the ticker is never started here, just as internal/api's own
// harness (internal/api/testutil_test.go) never started it either.
func (h *RouterHarness) Converge() {
	h.T.Helper()
	require.NoError(h.T, h.Engine.Tick(context.Background()))
}

// ValidateReverse replays the ARTP's act — engine.ValidateReverse, exposed
// here for the same reason as ContractFidelity: a controller test cannot
// import internal/framework/engine directly (layer 2 into layer 3, which
// test/architecture_test.go would forbid).
func (h *RouterHarness) ValidateReverse(reverseID string) {
	h.T.Helper()
	require.NoError(h.T, engine.ValidateReverse(context.Background(), h.DB, reverseID))
}

// Raw executes a raw HTTP request, without decoding the response.
func (h *RouterHarness) Raw(method, path, token string, payload any) *http.Response {
	h.T.Helper()
	var body *bytes.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(h.T, err)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, h.Srv.URL+path, body)
	require.NoError(h.T, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.T, err)
	h.T.Cleanup(func() { resp.Body.Close() })
	return resp
}

// Call executes an authenticated request and decodes the body into a map.
func (h *RouterHarness) Call(method, path, token string, payload any) (*http.Response, map[string]any) {
	h.T.Helper()
	resp := h.Raw(method, path, token, payload)
	var parsed map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	return resp, parsed
}

// List executes an authenticated GET whose data is an array. Promoted here
// (ruling R25) from reference_controller_test.go's local liste(), its first
// caller — a second capability (Task 12, the seven read-only queues) needing
// the exact same helper is the signal the ruling names for promoting rather
// than copying it again.
func (h *RouterHarness) List(path, token string) []any {
	h.T.Helper()
	resp, body := h.Call(http.MethodGet, path, token, nil)
	require.Equal(h.T, http.StatusOK, resp.StatusCode, path)
	data, ok := body["data"].([]any)
	require.Truef(h.T, ok, "%s: data is not an array (%v)", path, body)
	return data
}

// Token authenticates a seeded account and returns its id_token.
func (h *RouterHarness) Token(username, password string) string {
	h.T.Helper()
	resp := h.Raw(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": username, "password": password, "rememberMe": false,
	})
	require.Equal(h.T, http.StatusOK, resp.StatusCode)

	var body struct {
		IDToken string `json:"id_token"`
	}
	require.NoError(h.T, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(h.T, body.IDToken)
	return body.IDToken
}
