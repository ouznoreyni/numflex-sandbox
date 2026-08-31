package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestSoumissionReverseParLOperateurDOrigine(t *testing.T) {
	// Tranche 773 : YAS actuellement, ORANGE à l'origine.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	require.Equal(t, "Demande de reverse soumise avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, "773000001", data["numero"])
	require.Equal(t, "EN_ATTENTE", data["statut"])
	require.Equal(t, seed.OperateurOrange, data["operateur"].(map[string]any)["id"])
}

func TestSoumissionReverseParUnAutreOperateurRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestMesDemandesReverse(t *testing.T) {
	h := nouveauHarnais(t)
	h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	data := h.liste("/api/gateway/v1/reverse-requests/mes-demandes",
		h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, "EN_ATTENTE", data[0].(map[string]any)["statut"])

	require.Empty(t, h.liste("/api/gateway/v1/reverse-requests/mes-demandes",
		h.jeton("yas", "yas2026")))
}

func TestAucunEndpointDAnnulationDeReverse(t *testing.T) {
	// §7.6 : « Il n'existe pas d'endpoint pour annuler une demande de reverse. »
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep := h.brut(http.MethodPost, "/api/gateway/v1/reverse-requests/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}
