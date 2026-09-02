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

func TestCancelByCreator(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Token("yas", "yas2026"), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "Demande annulée avec succès", body["message"])

	data := body["data"].(map[string]any)
	require.Equal(t, "ANNULE", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "TERMINE", data["statutEtapeActuel"])
	require.NotNil(t, data["dateFinalisation"])
}

func TestCancelBySourceRefused(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Token("orange", "orange2026"), nil)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		body["detail"])
}

func TestCancelAfterAcceptanceRefused(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Token("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		body["detail"])
}

func TestCancelWithoutRequestBody(t *testing.T) {
	// §7.11: no body is required.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp := h.Raw(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Token("yas", "yas2026"), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCancelInContractMode(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.Token("orange", "orange2026"), nil)

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", body["code"])
}
