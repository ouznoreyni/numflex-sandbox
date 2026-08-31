package api

import (
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
