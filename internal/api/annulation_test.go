package api

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/stretchr/testify/require"
)

func TestAnnulationParLeCreateur(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Demande annulée avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, "ANNULE", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "TERMINE", data["statutEtapeActuel"])
	require.NotNil(t, data["dateFinalisation"])
}

func TestAnnulationParLaSourceRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		corps["detail"])
}

func TestAnnulationApresAcceptationRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		corps["detail"])
}

func TestAnnulationSansCorpsDeRequete(t *testing.T) {
	// §7.11 : aucun corps n'est requis.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep := h.brut(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestAnnulationEnModeContrat(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)

	require.Equal(t, http.StatusForbidden, rep.StatusCode)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", corps["code"])
}
