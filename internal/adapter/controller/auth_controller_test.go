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

func TestAuthentificationNominale(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	for _, c := range []struct{ user, pass string }{
		{"orange", "orange2026"},
		{"yas", "yas2026"},
		{"expresso", "expresso2026"},
	} {
		require.NotEmpty(t, h.Jeton(c.user, c.pass), c.user)
	}
}

func TestAuthentificationMauvaisMotDePasse(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": "yas", "password": "faux", "rememberMe": false,
	})
	// ANO-016 : l'échec d'authentification sort hors enveloppe.
	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.NotContains(t, corps, "success")
	require.NotContains(t, corps, "code")
}

func TestGetAuthenticateAvecJetonValide(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep := h.Brut(http.MethodGet, "/api/authenticate", h.Jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNoContent, rep.StatusCode)
}

func TestJetonAbsentRendEnveloppeAccesInterdit(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodGet, "/api/gateway/v1/operateurs", "", nil)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, false, corps["success"])
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		corps["message"])
}

func TestJetonInvalideRendCorpsVideSansContentType(t *testing.T) {
	// ANO-008 : jeton invalide → 401, corps vide, aucun Content-Type.
	h := routerharness.NewRouterHarness(t)
	rep := h.Brut(http.MethodGet, "/api/gateway/v1/operateurs", "jeton.bidon.xxx", nil)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, "", rep.Header.Get("Content-Type"))
	require.Equal(t, int64(0), rep.ContentLength)
}

func TestJetonPorteurVideRendEnveloppeAccesInterdit(t *testing.T) {
	// Le brief énonce : « aucun en-tête Authorization, ou un porteur vide ».
	// "Bearer " (rien après) doit produire le 401 enveloppé ACCES_INTERDIT,
	// pas le 401 à corps vide réservé au jeton invalide.
	h := routerharness.NewRouterHarness(t)
	req, err := http.NewRequest(http.MethodGet, h.Srv.URL+"/api/gateway/v1/operateurs", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer ")
	rep, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { rep.Body.Close() })

	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, false, corps["success"])
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		corps["message"])
}

func TestGardePrefixeNeCaptureNiPasUnCheminFrontalier(t *testing.T) {
	// La garde de préfixe compare le segment entier : /api/gateway/v1extra
	// n'est pas sous l'arbre protégé et doit ressortir en 404, pas en 401,
	// même sans jeton — sinon un chemin hors contrat futur (/api/gateway/v10)
	// serait à tort absorbé par le middleware d'authentification.
	h := routerharness.NewRouterHarness(t)
	rep := h.Brut(http.MethodGet, "/api/gateway/v1extra", "", nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}

func TestCheminGatewayInexistantAvecJetonValideRendQuandMeme404(t *testing.T) {
	// Garantie de périmètre du routeur : une requête authentifiée vers un
	// chemin non déclaré sous le préfixe gateway doit recevoir le 404 normal,
	// pas être absorbée par le middleware d'authentification.
	h := routerharness.NewRouterHarness(t)
	rep := h.Brut(http.MethodGet, "/api/gateway/v1/route-inexistante", h.Jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}
