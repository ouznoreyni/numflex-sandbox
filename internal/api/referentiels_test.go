package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperateursRenvoieLesTroisIdentifiantsDeRecette(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/operateurs", h.jeton("yas", "yas2026"))

	require.Len(t, data, 3)
	vus := map[string]string{}
	for _, e := range data {
		m := e.(map[string]any)
		require.Len(t, m, 2, "un opérateur ne porte que id et nom")
		vus[m["id"].(string)] = m["nom"].(string)
	}
	require.Equal(t, map[string]string{
		"6a21745ce6c37b5b5b487ec1": "ORANGE",
		"6a2174c3e6c37b5b5b487ec4": "YAS",
		"6a217510e6c37b5b5b487ec7": "EXPRESSO",
	}, vus)
}

func TestOperateursMessageExact(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel("GET", "/api/gateway/v1/operateurs", h.jeton("yas", "yas2026"), nil)
	require.Equal(t, "Opérateurs récupérés avec succès", corps["message"])
	require.Equal(t, "SUCCESS", corps["code"])
	require.Equal(t, true, corps["success"])
}

func TestMotifsRejetExposeMotifPasLibelle(t *testing.T) {
	// ANO-009 : le champ s'appelle motif. La v2 le documente ainsi.
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/motifs-rejet", h.jeton("orange", "orange2026"))

	require.Len(t, data, 6)
	premier := data[0].(map[string]any)
	require.Contains(t, premier, "motif")
	require.NotContains(t, premier, "libelle")
}

func TestTypesDemande(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/types-demande", h.jeton("yas", "yas2026"))

	types := []string{}
	for _, e := range data {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	require.ElementsMatch(t, []string{"PORTAGE", "RESTITUTION", "REVERSE"}, types)
}

func TestProcessus(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/processus", h.jeton("yas", "yas2026"))

	types := []string{}
	for _, e := range data {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	require.ElementsMatch(t, []string{"PREPAID", "POSTPAID"}, types)
}

func TestTypesIncidentPorteFigeSysteme(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/types-incident", h.jeton("yas", "yas2026"))

	require.Len(t, data, 2)
	par := map[string]bool{}
	for _, e := range data {
		m := e.(map[string]any)
		par[m["libelle"].(string)] = m["figeSysteme"].(bool)
	}
	require.Equal(t, map[string]bool{"Gateway": false, "Technique": true}, par)
}
