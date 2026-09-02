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
	Moteur *engine.Engine
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

// Convergence returns an ajuste function that fixes the engine's
// convergence window to [min, max] — the same reason FiabiliteContrat
// exists: a controller test cannot name internal/framework/config's
// *config.Config field directly, only through a func literal built here.
// A non-zero window makes PlanifierTransition defer the transition (R-10)
// rather than apply it synchronously within the request.
func Convergence(min, max time.Duration) func(*config.Config) {
	return func(c *config.Config) {
		c.ConvergenceMin, c.ConvergenceMax = min, max
	}
}

// CompletionLatency returns an ajuste function that fixes
// config.CompletionLatency (ANO-005) — the same reason FiabiliteContrat
// exists.
func CompletionLatency(d time.Duration) func(*config.Config) {
	return func(c *config.Config) { c.CompletionLatency = d }
}

// SandboxAdmin opens /api/sandbox/v1 for the harness's router — the same
// reason FiabiliteContrat exists: a controller test cannot name
// internal/framework/config's own SandboxAdmin field directly.
func SandboxAdmin(c *config.Config) { c.SandboxAdmin = true }

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
	d := &web.Deps{
		Cfg:    cfg,
		DB:     db,
		Moteur: mot,
	}
	srv := httptest.NewServer(web.NewRouter(d))
	t.Cleanup(srv.Close)

	return &RouterHarness{T: t, Srv: srv, DB: db, Cfg: cfg, Moteur: mot}
}

// Converger déclenche un passage du moteur et vérifie qu'aucune transition
// ne reste due. Les tests pilotent le moteur explicitement plutôt que
// d'attendre son ticker — celui-ci n'est jamais démarré ici, comme
// internal/api's own harnais (internal/api/testutil_test.go) ne le démarre
// pas non plus.
func (h *RouterHarness) Converger() {
	h.T.Helper()
	require.NoError(h.T, h.Moteur.Tick(context.Background()))
}

// ValiderReverse rejoue l'acte de l'ARTP — engine.ValiderReverse, exposé ici
// pour la même raison que FiabiliteContrat : un test de contrôleur ne peut
// pas importer internal/framework/engine directement (couche 2 vers couche
// 3, que test/architecture_test.go interdirait).
func (h *RouterHarness) ValiderReverse(reverseID string) {
	h.T.Helper()
	require.NoError(h.T, engine.ValiderReverse(context.Background(), h.DB, reverseID))
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

// Liste exécute un GET authentifié dont data est un tableau. Promoted here
// (ruling R25) from reference_controller_test.go's local liste(), its first
// caller — a second capability (Task 12, the seven read-only queues) needing
// the exact same helper is the signal the ruling names for promoting rather
// than copying it again.
func (h *RouterHarness) Liste(chemin, jeton string) []any {
	h.T.Helper()
	rep, corps := h.Appel(http.MethodGet, chemin, jeton, nil)
	require.Equal(h.T, http.StatusOK, rep.StatusCode, chemin)
	data, ok := corps["data"].([]any)
	require.Truef(h.T, ok, "%s : data n'est pas un tableau (%v)", chemin, corps)
	return data
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
