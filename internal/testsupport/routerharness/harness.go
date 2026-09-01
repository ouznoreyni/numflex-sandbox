// Package routerharness starts the real, live router for adapter-layer
// controller tests. It is a separate package from internal/testsupport
// (rather than living alongside NewTestDB there) specifically because it
// imports internal/api: internal/testsupport is itself imported by
// internal/api's own tests (for NewTestDB), and internal/api importing back
// from the same package would be a test import cycle. Only controller test
// packages (internal/adapter/controller and its successors) import
// routerharness; internal/api and internal/engine's own tests never do.
package routerharness

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/api"
	"github.com/ouznoreyni/numflex-sandbox/internal/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/httpx"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/stretchr/testify/require"
)

// RouterHarness starts the real, live router — api.NewRouter, wired exactly
// as cmd/server/main.go wires it — against a fresh, migrated, seeded test
// database. It exists so an adapter-layer controller's test (which cannot
// import internal/framework directly: see test/architecture_test.go) can
// still exercise the whole HTTP stack end to end, proving a request really
// reaches its controller rather than some leftover handler.
//
// internal/api/testutil_test.go keeps its own private harnais for
// internal/api's own tests, which are not layer-constrained; this type is
// the one every capability's moved controller test (Task 9 onward) should
// use instead of reimplementing it.
type RouterHarness struct {
	T   *testing.T
	Srv *httptest.Server
	DB  *persistence.DB
	Cfg *config.Config
}

// FiabiliteContrat switches the harness's fidelity mode to "contract" — the
// adjustment TestCreationParticulierEnModeContratRendUnCodeMetier and
// TestFlotteVideRenvoieFlotteVide need. It exists here, rather than being
// written inline as func(c *config.Config) { c.Fidelity = ... } at the call
// site, because a controller's own test package cannot import
// internal/framework/config directly (test/architecture_test.go's
// dependency rule applies to _test.go files too): routerharness already
// imports config for NewRouterHarness's own signature, so this is the one
// place that dependency is allowed to live.
func FiabiliteContrat(c *config.Config) {
	c.Fidelity = config.FidelityContract
}

// NewRouterHarness mounts the full router in a deterministic profile.
// ajuste lets a test override a default config value before the router is
// built.
func NewRouterHarness(t *testing.T, ajuste ...func(*config.Config)) *RouterHarness {
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
	for _, f := range ajuste {
		f(cfg)
	}

	mot := engine.New(cfg, db)
	d := &api.Deps{
		Cfg:    cfg,
		DB:     db,
		R:      httpx.NewRenderer(cfg.Fidelity, cfg.ClockSkew),
		Moteur: mot,
	}
	srv := httptest.NewServer(api.NewRouter(d))
	t.Cleanup(srv.Close)

	return &RouterHarness{T: t, Srv: srv, DB: db, Cfg: cfg}
}

// Brut exécute une requête HTTP brute, sans décoder la réponse.
func (h *RouterHarness) Brut(methode, chemin, jeton string, corps any) *http.Response {
	h.T.Helper()
	var body *bytes.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		require.NoError(h.T, err)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(methode, h.Srv.URL+chemin, body)
	require.NoError(h.T, err)
	req.Header.Set("Content-Type", "application/json")
	if jeton != "" {
		req.Header.Set("Authorization", "Bearer "+jeton)
	}
	rep, err := http.DefaultClient.Do(req)
	require.NoError(h.T, err)
	h.T.Cleanup(func() { rep.Body.Close() })
	return rep
}

// Appel exécute une requête authentifiée et décode le corps en map.
func (h *RouterHarness) Appel(methode, chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.T.Helper()
	rep := h.Brut(methode, chemin, jeton, corps)
	var decode map[string]any
	_ = json.NewDecoder(rep.Body).Decode(&decode)
	return rep, decode
}

// Jeton authentifie un compte du seed et retourne son id_token.
func (h *RouterHarness) Jeton(username, motDePasse string) string {
	h.T.Helper()
	rep := h.Brut(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": username, "password": motDePasse, "rememberMe": false,
	})
	require.Equal(h.T, http.StatusOK, rep.StatusCode)

	var corps struct {
		IDToken string `json:"id_token"`
	}
	require.NoError(h.T, json.NewDecoder(rep.Body).Decode(&corps))
	require.NotEmpty(h.T, corps.IDToken)
	return corps.IDToken
}
