package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthentificationNominale(t *testing.T) {
	h := nouveauHarnais(t)
	for _, c := range []struct{ user, pass string }{
		{"orange", "orange2026"},
		{"yas", "yas2026"},
		{"expresso", "expresso2026"},
	} {
		require.NotEmpty(t, h.jeton(c.user, c.pass), c.user)
	}
}

func TestAuthentificationMauvaisMotDePasse(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": "yas", "password": "faux", "rememberMe": false,
	})
	// ANO-016 : l'échec d'authentification sort hors enveloppe.
	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.NotContains(t, corps, "success")
	require.NotContains(t, corps, "code")
}

func TestGetAuthenticateAvecJetonValide(t *testing.T) {
	h := nouveauHarnais(t)
	rep := h.brut(http.MethodGet, "/api/authenticate", h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNoContent, rep.StatusCode)
}

func TestJetonAbsentRendEnveloppeAccesInterdit(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodGet, "/api/gateway/v1/operateurs", "", nil)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, false, corps["success"])
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		corps["message"])
}

func TestJetonInvalideRendCorpsVideSansContentType(t *testing.T) {
	// ANO-008 : jeton invalide → 401, corps vide, aucun Content-Type.
	h := nouveauHarnais(t)
	rep := h.brut(http.MethodGet, "/api/gateway/v1/operateurs", "jeton.bidon.xxx", nil)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, "", rep.Header.Get("Content-Type"))
	require.Equal(t, int64(0), rep.ContentLength)
}

func TestJetonPorteurVideRendEnveloppeAccesInterdit(t *testing.T) {
	// Le brief énonce : « aucun en-tête Authorization, ou un porteur vide ».
	// "Bearer " (rien après) doit produire le 401 enveloppé ACCES_INTERDIT,
	// pas le 401 à corps vide réservé au jeton invalide.
	h := nouveauHarnais(t)
	req, err := http.NewRequest(http.MethodGet, h.srv.URL+"/api/gateway/v1/operateurs", nil)
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
	h := nouveauHarnais(t)
	rep := h.brut(http.MethodGet, "/api/gateway/v1extra", "", nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}

func TestCheminGatewayInexistantAvecJetonValideRendQuandMeme404(t *testing.T) {
	// Garantie de périmètre du routeur : une requête authentifiée vers un
	// chemin non déclaré sous le préfixe gateway doit recevoir le 404 normal,
	// pas être absorbée par le middleware d'authentification.
	h := nouveauHarnais(t)
	rep := h.brut(http.MethodGet, "/api/gateway/v1/route-inexistante", h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}
