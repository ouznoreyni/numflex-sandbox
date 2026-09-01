package controller_test

// These 5 test functions are moved, unchanged in assertion, from the
// deleted internal/api/annulation_test.go (Task 15). They still exercise
// the real, live router — routerharness.NewRouterHarness wraps
// api.NewRouter, wired exactly as cmd/server/main.go wires it — so a green
// run here proves a real HTTP request to /demandes/:id/annuler goes through
// the new PortingController, CancelRequestInteractor (which itself
// delegates every rule to entity.CanCancel) and port.UnitOfWork, not
// through any leftover handler.

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

func TestAnnulationParLeCreateur(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Demande annulée avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, "ANNULE", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "TERMINE", data["statutEtapeActuel"])
	require.NotNil(t, data["dateFinalisation"])
}

func TestAnnulationParLaSourceRefusee(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Jeton("orange", "orange2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		corps["detail"])
}

func TestAnnulationApresAcceptationRefusee(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "DESACTIVATION")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		corps["detail"])
}

func TestAnnulationSansCorpsDeRequete(t *testing.T) {
	// §7.11 : aucun corps n'est requis.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep := h.Brut(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestAnnulationEnModeContrat(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.FiabiliteContrat)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Jeton("orange", "orange2026"), nil)

	require.Equal(t, http.StatusForbidden, rep.StatusCode)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", corps["code"])
}
